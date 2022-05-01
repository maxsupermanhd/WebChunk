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

package main

import (
	"flag"
	"log"
	"path"

	"github.com/maxsupermanhd/WebChunk/credentials"
	gmma "github.com/maxsupermanhd/go-mc-ms-auth"
)

var (
	out = flag.String("out", "./", "Where to write retrieved credentials, will be written as \"username.json\"")
	cid = flag.String("cid", "88650e7e-efee-4857-b9a9-cf580a00ef43", "Azure AppID")
)

func main() {
	log.Println("Starting up...")
	flag.Parse()
	ms, err := gmma.AuthMSdevice(*cid)
	if err != nil {
		log.Fatal(err)
	}
	s := &credentials.StoredMicrosoftCredentials{
		Microsoft:     ms,
		Minecraft:     gmma.MCauth{},
		MinecraftUUID: "",
	}
	log.Println("Getting XBOX Live token...")
	XBLa, err := gmma.AuthXBL(s.Microsoft.AccessToken)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Getting XSTS token...")
	XSTSa, err := gmma.AuthXSTS(XBLa)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Getting Minecraft token...")
	MCa, err := gmma.AuthMC(XSTSa)
	if err != nil {
		log.Fatal(err)
	}
	s.Minecraft = MCa
	log.Println("Getting Minecraft profile...")
	resauth, err := gmma.GetMCprofile(MCa.Token)
	if err != nil {
		log.Fatal(err)
	}
	resauth.AsTk = MCa.Token
	s.MinecraftUUID = resauth.UUID
	err = credentials.WriteCredentials(path.Join(*out, resauth.Name+".json"), s)
	if err != nil {
		log.Fatal(err)
	}
}
