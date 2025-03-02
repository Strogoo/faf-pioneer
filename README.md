# faf-pioneer
FAF ICE Adapter based on WebRTC and the Pion library

## Data flow

There is a continuous data flow between this application, the Forged Alliance game, the FAF client and (obviously) the whole stack of these 3 applications on other players computers.

### GpgNet Protocol rules
A message in the GpgNet protocol is built up as followed:

(Encoding of uint32 is always Little Endian)

```
<message>              ::= <command> <size> <chunks>
<command>              ::= <string>
<size>                 ::= uint32
<string>               ::= <size> []byte
<chunks>               ::= <chunk> | <chunk> <chunks>
<chunk>                ::= <type_int> uint32 | <type_string> <string> | <type_followup_string> <string>
<type_int>             ::= byte = 0x00
<type_string>          ::= byte = 0x01
<type_followup_string> ::= byte = 0x02
```

### Local launch order
(Work in Progress)
1. The FAF client launches this ICE adapter and waits for it to signal readiness.
    * In prior iterations this was due to the ICE adapter running an RPC server and the FAF client succesfully connecting to it. 
2. The FAF client launches the Forged Alliance game.
3. The Forged Alliance game connects to the TCP server for the GpgNet port, that the FAF client has pathed to both the ICE adapter and the game.
4. The Forged Alliance game send a `GameState=Idle` message to the ICE adapter.
5. The FAF client tells the ICE adapter to host or join a game.
6. The ICE adapter tells the game to open a lobby with a predefined udp port.
7. The FAF client tells the game through the ICE adapter whom to connect to.


## Glossary

Due to the long arching history of connectivity among the history of the Forged Alliance Forever, a confusing terminology has established.

* ICE Adapter
  * An adapter application between the FAF client and the game that orchestrates the communication so that connectivity to other players is done externally (in this iteration via WebRTC, in prior iteration via ICE and local socket handling).
* Icebreaker
  * The new signalling server.
  * Grants access to the list of available STUN and TURN servers along with session based credentials.
* Lobby server
  * The service that the FAF client uses to manage game lobbies and initiate information on who plays with whom. In prior ICE adapter iterations it also had the role of the signalling server proxied via the FAF client.
* RPC connection
  * In prior iterations the FAF client communicated with the ICE adapter via JSON-RPC 2.0
  * The ICE adapter was the host.
* GpgNet
  * A binary TCP protocol for exchanging messages between the game and the lobby server.
  * The ICE adapter is acting as a server and the game connects to it. The port is specified as launch parameter of both applications.
  * Due to WebRTC/ICE changing ports and ip addresses, the ICE adapter has to read, intercept and modify some of the messages.
  * The protocol can be extended for more messages via SIM mods or game patches. Thus the adapter needs to handle and pass through all unknown messages to ensure compatibility.

## Contribute

This is the first Go application in FAForever project. We have absolutely no Go experience so far. If we do not follow best practices and you have improvements, all help is welcome.