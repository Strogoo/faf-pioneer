package forgedalliance

import (
	"bufio"
	"faf-pioneer/util"
	"faf-pioneer/webrtc"
	"io"
	"log/slog"
	"net"
	"strconv"
)

type Peer interface {
	IsOfferer() bool
}

type GpgNetServer struct {
	peerHandler webrtc.PeerHandler
	port        uint
	tcpSocket   *net.Listener
	currentConn *net.Conn
	state       string
}

func NewGpgNetServer(peerManager webrtc.PeerHandler, port uint) *GpgNetServer {
	return &GpgNetServer{
		peerHandler: peerManager,
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
			slog.Error("Error accepting GpgNet connection:", util.ErrorAttr(err))
			continue
		}

		s.currentConn = &conn

		slog.Info("New GpgNet client connected", slog.Any("remoteAddress", conn.RemoteAddr()))

		// Wrap the connection in a buffered reader.
		bufferReader := bufio.NewReader(conn)
		faStreamReader := NewFaStreamReader(bufferReader)

		go func() {
			slog.Info("Waiting for incoming GpgNet messages from game")

			for {
				// Read one message from the connection.
				command, err := faStreamReader.ReadString()
				if err == io.EOF {
					slog.Info("Closing GpgNet connection from game (EOF reached)")
					return
				}
				if err != nil {
					slog.Error("Error parsing GpgNet command from game, closing connection",
						slog.Any("remoteAddress", conn.RemoteAddr()),
						util.ErrorAttr(err),
					)
					return
				}

				chunks, err := faStreamReader.ReadChunks()
				if err == io.EOF {
					slog.Info("Closing GpgNet connection from game (EOF reached)")
					return
				}
				if err != nil {
					slog.Error("Error parsing GpgNet command from game, closing connection",
						slog.Any("remoteAddress", conn.RemoteAddr()),
						util.ErrorAttr(err),
					)
					return
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

			slog.Info("Waiting for GpgNet messages to be forwarded to the game")

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
		slog.Info("Local GameState changed", slog.String("state", msg.State))
		s.state = msg.State
		break
	case *JoinGameMessage:
		slog.Info("Joining game (swapping the address/port)")
		s.peerHandler.AddPeerIfMissing(msg.RemotePlayerId)

		mappedAddress := JoinGameMessage{
			Command:           msg.Command,
			RemotePlayerLogin: msg.RemotePlayerLogin,
			RemotePlayerId:    msg.RemotePlayerId,
			Destination:       "127.0.0.1:" + strconv.Itoa(int(s.port)),
		}
		var mappedMsg GpgMessage = &mappedAddress
		return &mappedMsg
	case *ConnectToPeerMessage:
		slog.Info("Connecting to peer (swapping the address/port)")
		s.peerHandler.AddPeerIfMissing(msg.RemotePlayerId)

		mappedAddress := ConnectToPeerMessage{
			Command:           msg.Command,
			RemotePlayerLogin: msg.RemotePlayerLogin,
			RemotePlayerId:    msg.RemotePlayerId,
			Destination:       "127.0.0.1:" + strconv.Itoa(int(s.port)),
		}
		var mappedMsg GpgMessage = &mappedAddress
		return &mappedMsg
	default:
		slog.Debug("Message command ignored", slog.String("command", msg.GetCommand()))
	}

	return &msg
}
