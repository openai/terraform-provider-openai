package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func writeReleaseFixtureArchive(t *testing.T, path string) {
	t.Helper()

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	provider, err := writer.Create("terraform-provider-openai_v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Write(releaseFixtureExecutable(t)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, path, archive.Bytes())

	name := filepath.Base(path)
	document := map[string]any{
		"spdxVersion": "SPDX-2.3",
		"name":        name,
		"relationships": []map[string]string{{
			"spdxElementId": "SPDXRef-DOCUMENT", "relationshipType": "DESCRIBES",
			"relatedSpdxElement": "SPDXRef-provider",
		}},
		"packages": []map[string]string{
			{"SPDXID": "SPDXRef-provider", "name": name,
				"versionInfo": "sha256:" + fmt.Sprintf("%x", sha256.Sum256(archive.Bytes())),
				"supplier":    "NOASSERTION"},
			{"name": "github.com/openai/openai-go/v3", "versionInfo": "v3.51.0", "licenseConcluded": "Apache-2.0"},
			{"name": "github.com/hashicorp/go-plugin", "versionInfo": "v1.7.0"},
		},
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, path+".spdx.json", encoded)
}

func releaseFixtureExecutable(t *testing.T) []byte {
	t.Helper()

	modules := "path\tgithub.com/openai/terraform-provider-openai\n" +
		"mod\tgithub.com/openai/terraform-provider-openai\t(devel)\t\n" +
		"dep\tgithub.com/openai/openai-go/v3\tv3.51.0\th1:fixture\n" +
		"dep\tgithub.com/hashicorp/go-plugin\tv1.7.0\th1:fixture\n"
	modules = strings.Repeat("x", 16) + modules + strings.Repeat("x", 16)
	info := make([]byte, 32)
	copy(info, []byte("\xff Go buildinf:"))
	info[14], info[15] = 8, 2
	info = binary.AppendUvarint(info, uint64(len("go1.25.8")))
	info = append(info, "go1.25.8"...)
	info = binary.AppendUvarint(info, uint64(len(modules)))
	info = append(info, modules...)

	header := elf.Header64{
		Type: uint16(elf.ET_EXEC), Machine: uint16(elf.EM_X86_64),
		Version: uint32(elf.EV_CURRENT), Phoff: 64, Ehsize: 64, Phentsize: 56, Phnum: 1,
	}
	copy(header.Ident[:], []byte{0x7f, 'E', 'L', 'F',
		byte(elf.ELFCLASS64), byte(elf.ELFDATA2LSB), byte(elf.EV_CURRENT)})
	program := elf.Prog64{
		Type: uint32(elf.PT_LOAD), Flags: uint32(elf.PF_R | elf.PF_W),
		Off: 128, Vaddr: 0x400080, Filesz: uint64(len(info)), Memsz: uint64(len(info)), Align: 16,
	}
	var executable bytes.Buffer
	if err := binary.Write(&executable, binary.LittleEndian, header); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&executable, binary.LittleEndian, program); err != nil {
		t.Fatal(err)
	}
	executable.Write(make([]byte, 128-executable.Len()))
	executable.Write(info)
	return executable.Bytes()
}
