package routing

import (
	"log"
	"reflect"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/shinhagunn/websocket/pkg/message"
)

func getTopic(scope, stream, typ string) string {
	if scope == "private" {
		return typ
	}
	return stream + "." + typ
}

func splitPrefixedTopic(prefixed string) (string, string) {
	spl := strings.Split(prefixed, ".")
	prefix := spl[0]
	t := strings.TrimPrefix(prefixed, prefix+".")

	return prefix, t
}

func responseMust(e error, r interface{}) string {
	res, err := message.PackOutgoingResponse(e, r)
	if err != nil {
		log.Panic("responseMust failed:" + err.Error())
		panic(err.Error())
	}

	return string(res)
}

func isPrivateStream(s string) bool {
	return strings.Count(s, ".") == 0
}
func isPrefixedStream(s string) bool {
	return strings.Count(s, ".") == 2
}

func websocketFD(conn *websocket.Conn) int {
	connVal := reflect.Indirect(reflect.ValueOf(conn)).FieldByName("conn").Elem()
	tcpConn := reflect.Indirect(connVal).FieldByName("conn")
	fdVal := tcpConn.FieldByName("fd")
	pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")
	return int(pfdVal.FieldByName("Sysfd").Int())
}
