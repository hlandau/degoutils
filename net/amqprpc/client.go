package amqprpc

import "github.com/streadway/amqp"
import "gopkg.in/vmihailenco/msgpack.v2"
import "code.google.com/p/go-uuid/uuid"
import "sync"
import "time"
import "fmt"

// Client for doing msgpack-encoded JSON-RPC over AMQP.
type Client struct {
	conn               *amqp.Connection
	txChannel          *amqp.Channel
	rxChannel          *amqp.Channel
	rxQueueName        string
	rxCh               <-chan amqp.Delivery
	responseChans      map[string]chan amqp.Delivery
	responseChansMutex sync.Mutex
	closed             bool
}

// Creates a new client, connecting to the AMQP URL specified.  A nil
// amqp.Config may be specified for default connection parameters.
func NewClient(url string, cfg amqp.Config) (*Client, error) {
	var err error

	c := &Client{
		responseChans: map[string]chan amqp.Delivery{},
	}

	c.conn, err = amqp.DialConfig(url, cfg)
	if err != nil {
		return nil, err
	}

	c.txChannel, err = c.conn.Channel()
	if err != nil {
		return nil, err
	}

	c.rxChannel, err = c.conn.Channel()
	if err != nil {
		return nil, err
	}

	rxQueue, err := c.rxChannel.QueueDeclare(
		"",    // name
		false, // durable
		true,  // autodelete
		true,  // exclusive
		false, // nowait
		nil,
	)
	if err != nil {
		return nil, err
	}

	c.rxQueueName = rxQueue.Name

	c.rxCh, err = c.rxChannel.Consume(c.rxQueueName,
		"",    // consumer
		true,  // autoAck
		true,  // exclusive
		false, // nolocal
		false, // nowait
		nil,
	)
	if err != nil {
		return nil, err
	}

	go c.responseHandler()

	return c, nil
}

func (c *Client) Close() {
	if c.closed {
		return
	}

	c.conn.Close()
	c.responseChansMutex.Lock()
	for _, ch := range c.responseChans {
		close(ch)
	}
	c.responseChans = map[string]chan amqp.Delivery{}
	defer c.responseChansMutex.Unlock()
	c.closed = true
}

func (c *Client) registerResponseChan(cid string) (ch chan amqp.Delivery) {
	c.responseChansMutex.Lock()
	defer c.responseChansMutex.Unlock()
	ch = make(chan amqp.Delivery, 1)
	c.responseChans[cid] = ch
	return ch
}

func (c *Client) cancelResponseChan(cid string) {
	c.responseChansMutex.Lock()
	defer c.responseChansMutex.Unlock()
	delete(c.responseChans, cid)
}

func (c *Client) rhGetClearResponseChan(cid string) (ch chan amqp.Delivery, ok bool) {
	c.responseChansMutex.Lock()
	defer c.responseChansMutex.Unlock()
	ch, ok = c.responseChans[cid]
	if ok {
		delete(c.responseChans, cid)
	}
	return
}

func (c *Client) responseHandler() {
	for delivery := range c.rxCh {
		rch, ok := c.rhGetClearResponseChan(delivery.CorrelationId)
		if !ok {
			// ...
			continue
		}

		rch <- delivery
	}
}

type request struct {
	Method string                 `msgpack:"method"`
	Params map[string]interface{} `msgpack:"params"`
}

type response struct {
	Result map[string]interface{} `msgpack:"result"`
	Error  map[string]interface{} `msgpack:"error"`
}

// Represents a JSON-RPC level error.
type RPCError struct {
	Info map[string]interface{}
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error: %+v", e.Info)
}

var ErrTimeout = fmt.Errorf("timeout expired")

// Initiates an RPC call using the exchange, routing key, method and arguments
// specified. If the timeout is nonzero, and a response is not received within
// the timeout, ErrTimeout is returned (any reply received after that is
// discarded). If a JSON-RPC error occurs, the error returned will be of type
// *RPCError. On success, returns the result map.
func (c *Client) Call(exchange, routingKey, method string, args map[string]interface{}, timeout time.Duration) (map[string]interface{}, error) {
	req := request{
		Method: method,
		Params: args,
	}

	reqb, err := msgpack.Marshal(&req)
	if err != nil {
		return nil, err
	}

	cid := uuid.New()
	rch := c.registerResponseChan(cid)

	err = c.txChannel.Publish(exchange, routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:   "application/json-rpc+x-msgpack",
			CorrelationId: cid,
			ReplyTo:       c.rxQueueName,
			Body:          reqb,
		})
	if err != nil {
		c.cancelResponseChan(cid)
		return nil, err
	}

	var d amqp.Delivery
	if timeout == 0 {
		d = <-rch
	} else {
		select {
		case d = <-rch:
			break
		case <-time.After(timeout):
			// ...
			c.cancelResponseChan(cid)
			return nil, ErrTimeout
		}
	}

	var res response
	err = msgpack.Unmarshal(d.Body, &res)
	if err != nil {
		return nil, err
	}

	if len(res.Error) > 0 {
		return nil, &RPCError{res.Error}
	}

	return res.Result, nil
}
