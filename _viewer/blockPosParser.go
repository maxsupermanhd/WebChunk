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

type BlockPosParser struct{}

type BlockPositionData struct {
	X, Y, Z  int64
	Relative [3]bool
}

func (p BlockPosParser) WriteTo(w io.Writer) (int64, error) {
	return pk.Tuple{
		pk.Identifier("minecraft:block_pos"),
	}.WriteTo(w)
}

var reBlockPos = regexp.MustCompile(`^(~?)(-?\d*) (~?)(-?\d*) (~?)(-?\d*)`)

func (p BlockPosParser) Parse(cmd string) (left string, value command.ParsedData, err error) {
	m := reBlockPos.FindStringSubmatch(cmd)
	if m == nil {
		return cmd, nil, command.ParseErr{
			Pos: 0,
			Err: "Invalid block position.",
		}
	}
	d := BlockPositionData{}
	for r := 0; r < 3; r++ {
		d.Relative[r] = m[r*2+1] == "~"
	}
	x, err := strconv.ParseInt(m[2], 10, 64)
	if err != nil {
		return cmd, nil, command.ParseErr{
			Pos: 0,
			Err: err.Error(),
		}
	}
	d.X = x
	y, err := strconv.ParseInt(m[4], 10, 64)
	if err != nil {
		return cmd, nil, command.ParseErr{
			Pos: 0,
			Err: err.Error(),
		}
	}
	d.Y = y
	z, err := strconv.ParseInt(m[6], 10, 64)
	if err != nil {
		return cmd, nil, command.ParseErr{
			Pos: 0,
			Err: err.Error(),
		}
	}
	d.Z = z
	return cmd[len(m[0]):], d, nil
}
