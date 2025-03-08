package forgedalliance

import (
	"bufio"
	"faf-pioneer/webrtc"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
)

type GpgNetServer struct {
	peerManager *webrtc.PeerManager
	port        uint
	tcpSocket   *net.Listener
	currentConn *net.Conn
	state       string
}

func NewGpgNetServer(peerManager *webrtc.PeerManager, port uint) *GpgNetServer {
	return &GpgNetServer{
		peerManager: peerManager,
		port:        port,
		state:       "disconnected",
	}
}

func (s *GpgNetServer) Listen(gameToAdapter chan<- *GpgMessage, adapterToGame chan *GpgMessage) error {
	tcpSocket, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(int(s.port)))
	if err != nil {
		return err
	}

	s.tcpSocket = &tcpSocket

	for {
		conn, err := tcpSocket.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		s.currentConn = &conn

		fmt.Println("New client connected:", conn.RemoteAddr())

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

				parsedMsg := unparsedMsg.TryParse()

				parsedMsg = *s.ProcessMessage(parsedMsg)

				if parsedMsg != nil {
					gameToAdapter <- &parsedMsg
				}
			}
		}()

		go func() {
			bufferedWriter := bufio.NewWriter(conn)
			faStreamWriter := NewFaStreamWriter(bufferedWriter)

			log.Println("Waiting for GpgNet messages to be sent")

			for msg := range adapterToGame {
				faStreamWriter.WriteMessage(*msg)
			}
		}()
	}
}

func (s *GpgNetServer) Close() error {
	return (*s.tcpSocket).Close()
}

func (s *GpgNetServer) ProcessMessage(msg GpgMessage) *GpgMessage {
	switch msg := msg.(type) {
	case *GameStateMessage:
		log.Printf("Local GameState changed to %s\n", msg.State)
		s.state = msg.State
		break
	case *JoinGameMessage:
		log.Printf("Joining game (swapping the address/port)\n")
		s.peerManager.AddPeerIfMissing(msg.RemotePlayerId)

		mappedAddress := JoinGameMessage{
			Command:           msg.Command,
			RemotePlayerLogin: msg.RemotePlayerLogin,
			RemotePlayerId:    msg.RemotePlayerId,
			Destination:       "127.0.0.1:" + strconv.Itoa(int(s.port)),
		}
		var mappedMsg GpgMessage = &mappedAddress
		return &mappedMsg
	case *ConnectToPeerMessage:
		log.Printf("Connecting to peer (swapping the address/port)\n")
		s.peerManager.AddPeerIfMissing(msg.RemotePlayerId)

		mappedAddress := ConnectToPeerMessage{
			Command:           msg.Command,
			RemotePlayerLogin: msg.RemotePlayerLogin,
			RemotePlayerId:    msg.RemotePlayerId,
			Destination:       "127.0.0.1:" + strconv.Itoa(int(s.port)),
		}
		var mappedMsg GpgMessage = &mappedAddress
		return &mappedMsg
	default:
		log.Printf("Message command %s ignored\n", msg.GetCommand())
	}

	return &msg
}
