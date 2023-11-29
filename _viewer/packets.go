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
	"github.com/google/uuid"
	"github.com/maxsupermanhd/go-vmc/v764/chat"
	"github.com/maxsupermanhd/go-vmc/v764/data/packetid"
	pk "github.com/maxsupermanhd/go-vmc/v764/net/packet"
	"github.com/maxsupermanhd/go-vmc/v764/server"
)

func SendUnloadChunk(p *server.Client, x, z int32) {
	p.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundForgetLevelChunk,
		pk.Int(x), pk.Int(z),
	)))
}

func SendUpdateViewPosition(p *server.Client, x, z int32) {
	p.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundSetChunkCacheCenter,
		pk.VarInt(x), pk.VarInt(z),
	)))
}

func SendPlayerPositionAndLook(p *server.Client, x, y, z float32, yaw, pitch float32, flags int8, teleportid int32, dismount bool) {
	p.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundPlayerPosition,
		pk.Double(x), pk.Double(y), pk.Double(z),
		pk.Float(yaw), pk.Float(pitch),
		pk.Byte(flags),
		pk.VarInt(69),
		pk.Boolean(false),
	)))
}

func SendSetGamemode(p *server.Client, gamemode int) {
	p.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundGameEvent,
		pk.UnsignedByte(3),
		pk.Float(gamemode),
	)))
}

func SendInfoMessage(p *server.Client, msg chat.Message) {
	p.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundChat,
		msg,
		pk.Byte(2),
		pk.UUID(uuid.Nil),
	)))
}

func SendSystemMessage(p *server.Client, msg chat.Message) {
	p.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundChat,
		msg,
		pk.Byte(1),
		pk.UUID(uuid.Nil),
	)))
}

func SendPlayerAbilities(p *server.Client, invulnerable, flying, allowFlying, instabreak bool, flyingSpeed pk.Float, fovModifier pk.Float) {
	flags := pk.Byte(0)
	if invulnerable {
		flags += 0x01
	}
	if flying {
		flags += 0x02
	}
	if allowFlying {
		flags += 0x04
	}
	if instabreak {
		flags += 0x08
	}
	p.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundPlayerAbilities,
		flags,
		flyingSpeed,
		fovModifier,
	)))
}
