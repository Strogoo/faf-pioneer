package forgedalliance

import (
	"bufio"
	"faf-pioneer/util"
	"fmt"
	"io"
	"log/slog"
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
		slog.Info("Waiting for incoming GpgNet message from parent")

		for {
			// Read one message from the connection.
			command, err := faStreamReader.ReadString()
			if err == io.EOF {
				slog.Info("Closing GpgNet connection from parent (EOF reached)")
				return
			}
			if err != nil {
				slog.Error("Error parsing command from parent GpgNetServer, closing connection",
					slog.Any("remoteAddress", conn.RemoteAddr()),
					util.ErrorAttr(err),
				)
				return
			}

			chunks, err := faStreamReader.ReadChunks()
			if err == io.EOF {
				slog.Info("Closing GpgNet connection from parent (EOF reached)")
				return
			}
			if err != nil {
				slog.Error("Error parsing command from parent GpgNetServer, closing connection",
					slog.Any("remoteAddress", conn.RemoteAddr()),
					util.ErrorAttr(err),
				)
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

		slog.Info("Ready to forward GpgNet messages parent GpgNet server")

		for msg := range fafClientToAdapter {
			err := faStreamWriter.WriteMessage(*msg)
			if err != nil {
				slog.Error("Error writing command to parent GpgNetServer, closing connection",
					slog.Any("remoteAddress", conn.RemoteAddr()),
					slog.Any("message", msg),
					util.ErrorAttr(err),
				)
				return
			}
		}
	}()

	return nil
}

func (s *GpgNetClient) Close() {
	err := (*s.connection).Close()
	if err != nil {
		slog.Warn("Error on closing connection to parent GpgNet server",
			util.ErrorAttr(err),
		)
		return
	}
}
