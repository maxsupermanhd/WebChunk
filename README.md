<p align="center"><img src="splash.svg" width="680" height="200"></p>

![WebChunk with overworld section from Constantiam](preview.png)

WebChunk is a simple web server written in Go that works with Postgres datbase to store and render chunks from Minecraft to your browser.

It is mainly targeted at multiplayer anarchy servers because of "hacking" being a bold word that fits literally every game modification.

Designed to accept chunks from multiple players at once, provide very fast deserialization, storage and mapping information.

Join [discord server](https://discord.com/invite/DFsMKWJJPN) for more info!

## Features

- Map rendering:
  - [x] Colored terrain view
  - [x] Heightmap terrain view
  - [x] Portals/chests heatmap overlay
  - [x] Colors customization
  - [x] Biome view
  - [x] Serving images over HTTP
  - [x] Customizable overlaying
- Connectivity:
  - [x] Accepting chunks from HTTP
  - [x] Sniffing chunks through proxied connection
  - [x] Accepting compressed chunks
  - [x] Concurrent use
  - [x] Compatibility with Minecraft's region file format

[Complete roadmap](https://github.com/maxsupermanhd/WebChunk/blob/master/docs/roadmap.md)

## Data source

Currently storage interface operates with anvil chunk format that can be grabbed from both region files and game itself. WebChunk acts like a proxy for server connections and will sniff chunk information from the connection. World directories can be used directly. Storing multiple versions of same chunk is also permitted and viewed as a feature that can be further supported and used to analyze how terrain/world changed, potentially converting whole thing into data analysis framework (only with postgresql storage).

## Storage

WebChunk currently supports storing data in PostgreSQL database (empty template of schemas are located in `db/sql/init/init.sh`) and in region files. Work has been put into making storage interfacing not complex and as easy to implemet as possible, although it supports multiple worlds it is not mandatory to provide multi-world functionality or even more than one dimension. There are no plans on being able to store older chunk versions with filesystem storage.

## How does it work?

Upon deploying WebChunk to a server or starting it locally it will accept NBT serialized chunk information over HTTP endpoint and store it in attached Postgres database, this is basically it! Front page has a Leaflet map that requests images from WebChunk and displays them nicely and organized.

## Security and safety of the data

WebChunk is built to provide service to owner of the database and web server. It does not feature any user profiles, access rights or any other means of stopping other people from communicating with service. That directly means that you (owner of WebChunk deployment) in charge of regulating who can access resource. WebChunk may also have unknown vulnerabilities that can allow other people to gain unwanted access.

## License

### GNU Affero General Public License v3.0

See file `LICENSE` in the project root
