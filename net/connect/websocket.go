package connect

import "io"
import "fmt"
import "net"
import "net/http"
import "github.com/gorilla/websocket"

type wsFrameAdaptor struct {
	ws  *websocket.Conn
	res *http.Response
}

func (a *wsFrameAdaptor) ReadFrame() ([]byte, error) {
	_, b, err := a.ws.ReadMessage()
	return b, err
}

func (a *wsFrameAdaptor) WriteFrame(b []byte) error {
	return a.ws.WriteMessage(websocket.BinaryMessage, b)
}

func (a *wsFrameAdaptor) Close() error {
	return a.ws.Close()
}

func (a *wsFrameAdaptor) WSHTTPResponse() *http.Response {
	return a.res
}

func (a *wsFrameAdaptor) WSUnderlying() *websocket.Conn {
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

	return &wsFrameAdaptor{
		ws:  conn,
		res: res,
	}, nil
}

func init() {
	RegisterMethod("ws", true, wrapWS)
}
