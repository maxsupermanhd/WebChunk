# WebSocket interface

In order to accommodate fast 2 way connection between WebChunk sevrer and potential clients of any origin (in-game mod, browser or other)
a websocket interface was created.

WebChunk will send real-time information about other clients as events if connected client desires to get this information.

Examples of real time information from WebChunk to clients:

- List of currently connected players through WebSocket interfaces and proxies.
- Location and metadata about other clients (where players are and on what dimensions).
- Updates of visible map tiles to achieve real time terrain view.

All numbers in binary messages are BigEndian.

## Event definitions (s2c)

WebChunk sends both text and binary frames to the clients:

- Binary frame is used to update mapping information (pngs of regions) or to potentially provide chunks themselves (to avoid base64 or other escaping)
- Text frames are always JSONs that contain `Action` and action-specific filds

### s2c Text messages

#### `bulkPlayerUpdate`

```json
{
    "Action": "bulkPlayerUpdate",
    "Data": {
        "FlexCoral": {
            "X": 69,
            "Y": 420,
            "Z": 69,
            "World": "constantiam.net",
            "Dimension": "overworld"
        }
    }
}
```

#### `updateWorldsAndDims`

```json
{
    "Action": "updateWorldsAndDims",
    "Data": {
        "constantiam.net": ["overworld", "the_nether", "the_end"]
    }
}
```

#### `updateLayers`

```json
{
    "Action": "updateLayers",
    "Data": [
        {
            "Name":"terrain",
            "DisplayName":"Terrain",
            "IsOverlay":false,
            "IsDefault":false
        }
    ]
}
```

#### `message`

Just a service message from the server, for example notifying that error occured or player joined/left or potentially other info that user should be aware of (should be displayed in form of a log on the client)

Examples:

```json
{
    "Action": "message",
    "Data": "You are now connected to WebChunk 0.0.0 ffffffff 20230101.042000 go1.20"
}
```

```json
{
    "Action": "message",
    "Data": "FlexCoral joined constantiam.net"
}
```

### s2c Binary messages

All messages start with 1 byte opcode

#### `0x01` update map tile

In order to get this message client must provide it's viewport first! (imagine 50 players in different places and your client will get all their image updates)

Example:

```hex
01 (uint8, op code)
0000 000f (uint32, length of world name)
636f 6e73 7461 6e74 6961 6d2e 6e65 74 (world name bytes excluding null byte at the end)
0000 0009 (uint32, length of dimension name)
6f76 6572 776f 726c 64 (dimension name)
0000 000d (uint32, length of layer name)
7368 6164 6564 7465 7272 6169 6e (layer name)
05 0000 0000 (uint8, int32, int32: scale, x and z of the imagery, same as in http)
png data...
```

## Command definitions (c2s)

### c2s Text messages

They have same general structure as s2c events but just sent other way around

#### `updatePosition`

```json
{
    "Action": "updatePosition",
    "Data": {
        "Player": "FlexCoral",
        "World": "constantiam.net",
        "Dimension": "overworld",
        "X": 69,
        "Y": 420,
        "Z": 69
    }
}
```

#### `requestWorldsAndDims`

```json
{
    "Action": "requestWorldsAndDims"
}
```

#### `tileSubscribe`

```json
{
    "Action": "tileSubscribe",
    "Data": {
        "World": "constantiam.net",
        "Dimension": "overworld",
        "Layer": "shadedterrain",
        "S": 5,
        "X": -1,
        "Z": -2,
    }
}
```

#### `tileUnsubscribe`

Same data as `tileSubscribe`

#### `unsubscribeAll`

```json
{
    "Action": "unsubscribeAll",
    "Data": null,
}
```

#### `resubWorldDimension`

Tells server to resubscribe all currently subscribed tiles to given world+dimension.
Not ment to be used while subscribed to multiple worlds+dimensions at the same time.
Cut to not spam resub messages when client map switches dimension/world.
Server will re-send tile contents (same as regular update).

```json
{
    "Action": "resubWorldDimension",
    "Data": {
        "World": "constantiam.net",
        "Dimension": "overworld",
    }
}
```

### c2s Binary messages

#### `0x01` send chunk

```hex
01 (uint8, op code)
0000 000f (uint32, length of world name)
636f 6e73 7461 6e74 6961 6d2e 6e65 74 (world name bytes excluding null byte at the end)
0000 0009 (uint32, length of dimension name)
6f76 6572 776f 726c 64 (dimension name)
0000 0000 (int32, int32: x and z of the chunk)
chunk data... (starting with compression byte)
```
