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

var BlockEntityTypes = map[string]int32{
	"furnace":           0,
	"chest":             1,
	"trapped_chest":     2,
	"ender_chest":       3,
	"jukebox":           4,
	"dispenser":         5,
	"dropper":           6,
	"sign":              7,
	"mob_spawner":       8,
	"piston":            9,
	"brewing_stand":     10,
	"enchanting_table":  11,
	"end_portal":        12,
	"beacon":            13,
	"skull":             14,
	"daylight_detector": 15,
	"hopper":            16,
	"comparator":        17,
	"banner":            18,
	"structure_block":   19,
	"end_gateway":       20,
	"command_block":     21,
	"shulker_box":       22,
	"bed":               23,
	"conduit":           24,
	"barrel":            25,
	"smoker":            26,
	"blast_furnace":     27,
	"lectern":           28,
	"bell":              29,
	"jigsaw":            30,
	"campfire":          31,
	"beehive":           32,
}
