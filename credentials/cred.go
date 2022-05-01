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

package credentials

import (
	"encoding/json"
	"os"
	"path"
	"time"

	"github.com/Tnze/go-mc/bot"
	gmma "github.com/maxsupermanhd/go-mc-ms-auth"
)

type MicrosoftCredentialsManager struct {
	Root  string
	AppID string
}

func NewMicrosoftCredentialsManager(root string, appid string) *MicrosoftCredentialsManager {
	return &MicrosoftCredentialsManager{Root: root, AppID: appid}
}

type StoredMicrosoftCredentials struct {
	Microsoft     gmma.MSauth
	Minecraft     gmma.MCauth
	MinecraftUUID string
}

func (c *MicrosoftCredentialsManager) GetFilePath(username string) string {
	return path.Join(c.Root, username+".json")
}

func ReadCredentials(path string) (*StoredMicrosoftCredentials, error) {
	filebytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s StoredMicrosoftCredentials
	err = json.Unmarshal(filebytes, &s)
	return &s, err
}

func WriteCredentials(path string, cred *StoredMicrosoftCredentials) error {
	filebytes, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	return os.WriteFile(path, filebytes, 0600)
}

func (c *MicrosoftCredentialsManager) GetAuthForUsername(username string) (*bot.Auth, error) {
	s, err := ReadCredentials(c.GetFilePath(username))
	if err != nil {
		return nil, err
	}
	if s.Minecraft.ExpiresAfter-3 > time.Now().Unix() {
		err = gmma.CheckRefreshMS(&s.Microsoft, c.AppID)
		if err != nil {
			return nil, err
		}
		XBLa, err := gmma.AuthXBL(s.Microsoft.AccessToken)
		if err != nil {
			return nil, err
		}
		XSTSa, err := gmma.AuthXSTS(XBLa)
		if err != nil {
			return nil, err
		}
		MCa, err := gmma.AuthMC(XSTSa)
		if err != nil {
			return nil, err
		}
		s.Minecraft = MCa
		resauth, err := gmma.GetMCprofile(MCa.Token)
		if err != nil {
			return nil, err
		}
		resauth.AsTk = MCa.Token
		s.MinecraftUUID = resauth.UUID
		return &resauth, WriteCredentials(c.GetFilePath(resauth.Name), s)
	} else {
		return &bot.Auth{
			Name: username,
			UUID: s.MinecraftUUID,
			AsTk: s.Minecraft.Token,
		}, nil
	}
}
