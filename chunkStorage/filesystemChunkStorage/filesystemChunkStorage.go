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

package FilesystemChunkStorage

type FilesystemChunkStorage struct {
	Root string
}

func NewFilesystemChunkStorage(root string) (*FilesystemChunkStorage, error) {
	storage := FilesystemChunkStorage{Root: root}
	_, err := storage.ListWorlds()
	return &storage, err
}

func (s *FilesystemChunkStorage) Close() error {
	return nil
}
