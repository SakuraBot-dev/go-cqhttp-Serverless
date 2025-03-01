// +build linux windows,!arm darwin
// +build 386 amd64 arm arm64
// +build !race

package codec

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/pkg/errors"
	"github.com/wdvxdr1123/go-silk"
)

const silkCachePath = "/mnt/data/cache"

// EncodeToSilk 将音频编码为Silk
func EncodeToSilk(record []byte, tempName string, useCache bool) (silkWav []byte, err error) {
	// 1. 写入缓存文件
	rawPath := path.Join(silkCachePath, tempName+".wav")
	err = ioutil.WriteFile(rawPath, record, os.ModePerm)
	if err != nil {
		return nil, errors.Wrap(err, "write temp file error")
	}
	defer os.Remove(rawPath)

	// 2.转换pcm
	pcmPath := path.Join(silkCachePath, tempName+".pcm")
	cmd := exec.Command("ffmpeg", "-i", rawPath, "-f", "s16le", "-ar", "24000", "-ac", "1", pcmPath)
	if err = cmd.Run(); err != nil {
		return nil, errors.Wrap(err, "convert pcm file error")
	}
	defer os.Remove(pcmPath)

	// 3. 转silk
	pcm, err := ioutil.ReadFile(pcmPath)
	if err != nil {
		return nil, errors.Wrap(err, "read pcm file err")
	}
	silkWav, err = silk.EncodePcmBuffToSilk(pcm, 24000, 24000, true)
	if err != nil {
		return nil, errors.Wrap(err, "silk encode error")
	}
	if useCache {
		silkPath := path.Join(silkCachePath, tempName+".silk")
		err = ioutil.WriteFile(silkPath, silkWav, 0o666)
	}
	return
}

// RecodeTo24K 将silk重新编码为 24000 bit rate
func RecodeTo24K(data []byte) []byte {
	pcm, err := silk.DecodeSilkBuffToPcm(data, 24000)
	if err != nil {
		panic(err)
	}
	data, err = silk.EncodePcmBuffToSilk(pcm, 24000, 24000, true)
	if err != nil {
		panic(err)
	}
	return data
}
