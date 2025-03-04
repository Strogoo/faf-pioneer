package forgedalliance

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
)

type GpgNetClient struct {
	port       uint
	connection *net.Conn
	state      string
}

func NewGpgNetClient(port uint) *GpgNetClient {
	return &GpgNetClient{
		port: port,
	}
}

func (s *GpgNetClient) Listen(adapterToFafClient chan *GpgMessage, fafClientToAdapter chan *GpgMessage) error {
	conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(int(s.port)))
	if err != nil {
		return err
	}

	s.connection = &conn

	fmt.Println("Connected to parent GpgNetServer:", conn.RemoteAddr())

	// Wrap the connection in a buffered reader.
	bufferReader := bufio.NewReader(conn)
	faStreamReader := NewFaStreamReader(bufferReader)

	go func() {
		log.Println("Waiting for incoming GpgNet message")

		for {
			// Read one message from the connection.
			command, err := faStreamReader.ReadString()
			if err == io.EOF {
				fmt.Println("EOF reached, closing connection.")
				return
			}
			if err != nil {
				fmt.Printf("error parsing command from %s: %v\n", conn.RemoteAddr(), err)
				continue
			}

			chunks, err := faStreamReader.ReadChunks()
			if err == io.EOF {
				fmt.Println("EOF reached, closing connection.")
				return
			}
			if err != nil {
				fmt.Printf("error parsing command from %s: %v", conn.RemoteAddr(), err)
				continue
			}

			unparsedMsg := GenericGpgMessage{
				Command: command,
				Args:    chunks,
			}

			// WTF is going on here?
			var typed GpgMessage = &unparsedMsg

			adapterToFafClient <- &typed
		}
	}()

	go func() {
		bufferedWriter := bufio.NewWriter(conn)
		faStreamWriter := NewFaStreamWriter(bufferedWriter)

		log.Println("Waiting for GpgNet messages to be sent")

		for msg := range fafClientToAdapter {
			faStreamWriter.WriteMessage(*msg)
		}
	}()

	return nil
}

func (s *GpgNetClient) Close() {
	(*s.connection).Close()
}
