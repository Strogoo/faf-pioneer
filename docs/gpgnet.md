# GPGnet Protocol

## Overview
The GPGnet protocol is used to instruct the `Forged Alliance.exe` binary with information about what lobby to open, which players to connect, game options, game results and potentially much more.

GPGnet is used across the apps from game to lobby server:
```
[FA.exe] <--> [faf-pioneer] <--> [faf-client] <--> [faf-lobby-server]
```

The correct order of sending is crucial in order to get players into a lobby and connected to each other.

But the GPGnet protocol can also be extended by SIM mods in the game since the message are sent in Lua code. However: A running game can never receive or process GPGnet messages, once start, it is sending only.

## Binary representation

The game sends and receives GPGnet messages in a custom representation. The encoding is always Little Endian (but affects only uint32 fragments).

Here is the specification in Backus-Naur-Form:
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

## Known messages types

| **command**        | **sent by** | **args** | **index** | **type** | **content**                                  |
|--------------------|-------------|----------|-----------|----------|----------------------------------------------|
| GameState          | game        | 1        | 0         | str      | state: [Idle,Lobby, ???]                     |
| CreateLobby        | client      | 5        | 0         | int      | lobby init mode 0 = Normal / Custom 1 = Auto |
|                    |             |          | 1         | int      | lobby port                                   |
|                    |             |          | 2         | str      | local player name                            |
|                    |             |          | 3         | int      | local player id                              |
|                    |             |          | 4         | int      | unknown                                      |
| HostGame           | client      | 1        | 0         | str      | map name                                     |
| JoinGame           | client      | 3        | 0         | str      | net address                                  |
|                    |             |          | 1         | str      | remote player name                           |
|                    |             |          | 2         | int      | remote player id                             |
| ConnectToPeer      | client      | 3        | 0         | str      | net address                                  |
|                    |             |          | 1         | str      | remote player name                           |
|                    |             |          | 2         | int      | remote player id                             |
| DisconnectFromPeer | client      | 1        | 0         | int      | remote player id                             |
| GameOptions        | both?       | ?        | ?         | ?        | ?                                            |
| GameEndedMessage   | game        | 0        |           |          |                                              |


## Message flow for opening a lobby

t.b.d.