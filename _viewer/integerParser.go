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
	"io"
	"regexp"
	"strconv"

	pk "github.com/maxsupermanhd/go-vmc/v764/net/packet"
	"github.com/maxsupermanhd/go-vmc/v764/server/command"
)

type IntegerParser struct {
	Flags pk.Byte
	Min   pk.Int
	Max   pk.Int
}

func NewIntegerParser(min, max int32) IntegerParser {
	return IntegerParser{
		Flags: 0,
		Min:   pk.Int(min),
		Max:   pk.Int(max),
	}
}

func (p IntegerParser) WriteTo(w io.Writer) (int64, error) {
	if p.Flags&0x01 == 1 && p.Flags&0x02 == 1 {
		return pk.Tuple{
			pk.Identifier("brigadier:integer"),
			p.Flags,
			p.Min,
			p.Max,
		}.WriteTo(w)
	} else if p.Flags&0x01 == 1 && p.Flags&0x02 == 0 {
		return pk.Tuple{
			pk.Identifier("brigadier:integer"),
			p.Flags,
			p.Min,
		}.WriteTo(w)
	} else if p.Flags&0x01 == 0 && p.Flags&0x02 == 1 {
		return pk.Tuple{
			pk.Identifier("brigadier:integer"),
			p.Flags,
			p.Max,
		}.WriteTo(w)
	} else {
		return pk.Tuple{
			pk.Identifier("brigadier:integer"),
			p.Flags,
		}.WriteTo(w)
	}
}

var reInteger = regexp.MustCompile(`^(-?\d*)`)

func (p IntegerParser) Parse(cmd string) (left string, value command.ParsedData, err error) {
	m := reInteger.FindStringSubmatch(cmd)
	if m == nil {
		return cmd, nil, command.ParseErr{
			Pos: 0,
			Err: "Invalid integer.",
		}
	}
	r, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return cmd, nil, command.ParseErr{
			Pos: 0,
			Err: err.Error(),
		}
	}
	return cmd[len(m[0]):], r, nil
}
