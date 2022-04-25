/*
	WebChunk, web server for block game maps
	Copyright (C) 2022 Maxim Zhuchkov

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU Affero General Public License as published
	by the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU Affero General Public License for more details.

	You should have received a copy of the GNU Affero General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.

	Contact me via mail: q3.max.2011@yandex.ru or Discord: MaX#6717
*/

package viewer

import (
	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/server"
	"github.com/google/uuid"
)

func SendUnloadChunk(p *server.Player, x, z int32) {
	p.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundForgetLevelChunk,
		pk.Int(x), pk.Int(z),
	)))
}

func SendUpdateViewPosition(p *server.Player, x, z int32) {
	p.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundSetChunkCacheCenter,
		pk.VarInt(x), pk.VarInt(z),
	)))
}

func SendPlayerPositionAndLook(p *server.Player, x, y, z float32, yaw, pitch float32, flags int8, teleportid int32, dismount bool) {
	p.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundPlayerPosition,
		pk.Double(x), pk.Double(y), pk.Double(z),
		pk.Float(yaw), pk.Float(pitch),
		pk.Byte(flags),
		pk.VarInt(69),
		pk.Boolean(false),
	)))
}

func SendSetGamemode(p *server.Player, gamemode int) {
	p.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundGameEvent,
		pk.UnsignedByte(3),
		pk.Float(gamemode),
	)))
}

func SendChatMessage(p *server.Player, msg chat.Message) {
	p.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundChat,
		msg,
		pk.Byte(2),
		pk.UUID(uuid.Nil),
	)))
}
