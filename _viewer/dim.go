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

import "github.com/maxsupermanhd/go-vmc/v764/server"

type dimensionProvider struct {
	dbDimName  string
	hashedSeed uint64
}

func (s *dimensionProvider) Init(g *server.Game)         {}
func (s *dimensionProvider) PlayerJoin(p *server.Player) {}
func (s *dimensionProvider) PlayerQuit(p *server.Player) {}
func (s *dimensionProvider) Info() server.LevelInfo {
	return server.LevelInfo{
		Name:       "minecraft:" + s.dbDimName,
		HashedSeed: s.hashedSeed,
	}
}
