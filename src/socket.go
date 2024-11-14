package socket

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"maps"
	"math"
	"net"
	"net/http"
	"slices"
	"strings"
)

type Connection struct {
	net.Conn
	id int
}

var idCounter = 0

var connections = []*Connection{}

func Start(msgs chan string) {

	ln, err := net.Listen("tcp", ":6969")

	if err != nil {
		log.Fatal("erroring creating listener: ", err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			// handle error
		}
		idCounter++
		con := Connection{Conn: conn, id: idCounter}
		fmt.Println("connecting", con.id)
		connections = append(connections, &con)
		go HandleConnection(&con, msgs)
	}
}

type Response struct {
	Status  int
	Headers map[string]string
	Body    []byte
}

func (res *Response) AddHeader(header string, val string) {
	res.Headers[header] = val
}

func (res *Response) Convert() []byte {
	var header strings.Builder
	fmt.Fprintf(&header, "HTTP/1.1 %d %s\r\n", res.Status, http.StatusText(res.Status))
	for k, v := range maps.All(res.Headers) {
		fmt.Fprintf(&header, "%s: %s\r\n", k, v)
	}

	fmt.Fprintf(&header, "Content-Length: %d\r\n", len(res.Body))
	fmt.Fprintf(&header, "\r\n")
	headerByte := []byte(header.String())
	body := append(headerByte, res.Body...)
	return body
}

func SendMsg(conn *Connection, msg string) {
	b := []byte{
		0b1000_0001,
	}
	if len(msg) < 126 {
		b = append(b, byte(len(msg)))
	} else if len(msg) < int(math.Pow(2, 16)) {
		b = append(b, 0b0111_1110)
		b = append(b, byte(len(msg)>>8))
		b = append(b, byte(len(msg)%256))
	} else if len(msg) < int(math.Pow(2, 64)) {
		b = append(b, 0b0111_1111)
		b = append(b, byte(len(msg)>>32))
		b = append(b, byte(len(msg)>>16))
		b = append(b, byte(len(msg)>>8))
		b = append(b, byte(len(msg)%256))
	}

	b = append(b, []byte(msg)...)
	(*conn).Write(b)
}

func HandleWebsocket(reader *bufio.Reader, conn *Connection, msgs chan string) {

	for m := range msgs {
		SendMsg(conn, m)
	}

	b, _ := reader.ReadByte()
	if b > 127 {
		fmt.Println("closing", conn.id)
		conn.Close()
		connections = slices.DeleteFunc(connections, func(c *Connection) bool {
			return c.id == conn.id
		})
	}

}

func HandleConnection(conn *Connection, msgs chan string) {
	// defer conn.Close()
	reader := bufio.NewReader(*conn)
	_, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal("erroring creating listener: ", err)
	}
	headers := make(map[string]string)
	for true {
		headerLine, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal("erroring creating listener: ", err)
		}
		headerLine = strings.TrimSpace(headerLine)
		if len(headerLine) == 0 {
			break
		}
		splitted := strings.SplitN(headerLine, ":", 2)
		headers[splitted[0]] = strings.TrimSpace(splitted[1])
	}

	key := headers["Sec-WebSocket-Key"]
	h := sha1.New()
	io.WriteString(h, key+"258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
	sum := h.Sum(nil)

	res := Response{
		Status: 101,
		Headers: map[string]string{
			"Connection":           "Upgrade",
			"Upgrade":              "websocket",
			"Sec-WebSocket-Accept": base64.StdEncoding.EncodeToString(sum),
		},
		Body: []byte(""),
	}

	(*conn).Write(res.Convert())

	HandleWebsocket(reader, conn, msgs)
}
