# Configuration

WebChunk uses JSON for configuration file.\
WebChunk will likely (partially) shut down or panic if configuration is incorrect (likely on startup).\
Configuration tree is populated with default values if values are not found, it allows
starting of WebChunk without config file and later creation/saving defaulted configuration.\
Path to load config from is taken from environment variable `WEBCHUNK_CONFIG` and defaults to `config.json` if empty.

## Configuration tree

| Path | Type | Can be changed | Default | Description |
| --- | --- | --- | --- | --- |
| `logs_path` | string | No | `./logs/WebChunk.log` | Path to log file (will create files and directories if needed) |
| `colors_path` | string | Yes 🔧 |`./colors.gob` | Path to GOB-encoded block color palette |
| `ignore_failed_storages` | bool | No | `false` | Continue to start webchunk if errors occur on storages init |
| `storages` | object | No | `{}` | Contains defined storages, see [Storage object](#storage-object) |
| `render_received` | bool | Yes | `true` | Do render chunks immediately when received |
| `imaging_workers` | int | No | `4` | Essentially number of IO threads that read/write from cache |
| `cache_path` | string | Yes | `imageCache` | Path to where cached images should be stored |
| `max_memory_image_cache` | int | No | `512` | Number of images to cache (each image is 512x512 taking a bit more than 1 megabyte of memory) |
| `web` | object | Parially | see below | Group for web-related parameters |
| `web`.`listen_addr` | string | No | `localhost:3002` | Web server listen address |
| `web`.`templates_glob` | string | Yes | `./templates/*.gohtml` | Glob for HTML templates |
| `web`.`template_reload` | bool | No | `false` | Automatically reload HTML templates if changes detected (for development) |
| `proxy` | object | Parially | see below | Group for proxy-related parameters |
| `proxy`.`listen_addr` | string | No | `localhost:25566` | Proxy server listen address |
| `proxy`.`icon_path` | string | No | empty | Path to icon for the proxy server query response (can be empty) |
| `proxy`.`max_players` | int | No | `999` | Maximum player count for the proxy server query response (afaik does not actually limit proxied players count) |
| `proxy`.`motd` | chat JSON | No | `{"text": "WebChunk proxy"}` | Message for the proxy server query response (follows Mojang's chat JSON structure) |
| `proxy`.`online_mode` | bool | No | `true` | Same as online-mode on regular Minecraft servers |
| `proxy`.`compress_threshold` | int | No | `-1` | Threshold set the smallest size of raw network payload to compress. Set to 0 to compress all packets. Set to -1 to disable compression. |
| `proxy`.`routes` | object | Yes | `{}` | Place for routing rules of players connecting to proxy (example: `{"FlexCoral": "constantiam.net"}`) |
| `proxy`.`credentials_path` | string | No | `./cmd/auth/` | Path to credentials directory |

🔧 - Asociated system must be reloaded manually

### Storage object

Storage object contains 2 fields: `type` and `address`.

Storage types:

- `postgres` PostgreSQL database, address is a URI or DSN connection string to the database
- `filesystem` Mojang-compatible anvil region format storage, address is a path to the directory (will not be created automatically)

Example of storage objects:

```json
{
    "storages": {
        "default": {
            "type": "filesystem",
            "address": "./storages/defalt"
        },
        "database": {
            "type": "postgres",
            "address": "host=localhost dbname=chunkdb user=webchunk password=chunky81254 port=9182 connect_timeout=3"
        }
    }
}
```
