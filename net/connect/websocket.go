package connect

import "io"
import "fmt"
import "net"
import "net/http"
import "github.com/gorilla/websocket"

type WSFrameAdaptor struct {
	ws  *websocket.Conn
	req *http.Request
	res *http.Response
}

func NewWSFrameAdaptor(ws *websocket.Conn, req *http.Request, res *http.Response) *WSFrameAdaptor {
	return &WSFrameAdaptor{
		ws:  ws,
		req: req,
		res: res,
	}
}

func (a *WSFrameAdaptor) ReadFrame() ([]byte, error) {
	_, b, err := a.ws.ReadMessage()
	return b, err
}

func (a *WSFrameAdaptor) WriteFrame(b []byte) error {
	return a.ws.WriteMessage(websocket.BinaryMessage, b)
}

func (a *WSFrameAdaptor) Close() error {
	return a.ws.Close()
}

func (a *WSFrameAdaptor) WSHTTPRequest() *http.Request {
	return a.req
}

func (a *WSFrameAdaptor) WSHTTPResponse() *http.Response {
	return a.res
}

func (a *WSFrameAdaptor) WSUnderlying() *websocket.Conn {
	return a.ws
}

func wrapWS(c io.Closer, info *MethodInfo) (io.Closer, error) {
	hdrs, ok := info.Pragma["ws-headers"].(http.Header)
	if !ok {
		hdrs = http.Header{}
	}

	co, ok := c.(net.Conn)
	if !ok {
		return nil, fmt.Errorf("Websocket requires net.Conn")
	}

	conn, res, err := websocket.NewClient(co, info.URL, hdrs, 0, 0)
	if err != nil {
		return nil, err
	}

	return NewWSFrameAdaptor(conn, nil, res), nil
}

func init() {
	RegisterMethod("ws", true, wrapWS)
}
