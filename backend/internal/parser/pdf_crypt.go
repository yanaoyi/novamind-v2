package parser

// PDF 标准安全处理器（Standard Security Handler）解密。
//
// 为什么必须做：大量中文 PDF（出版社排版、超星/方正导出）只用**所有者密码**加密，
// 用户密码为空 —— 任何阅读器都能直接打开看，但字节流是加密的。
// 不做这一步，这类书会被误判成"打不开的加密文件"（实测董秘信息披露手册 26 万字、冷读术 5.7 万字都栽在这）。
//
// 覆盖：V1/V2（RC4 40/128 位）、V4（RC4 或 AESV2 128 位）。
// 不覆盖：V5/R5/R6（AES-256）—— 明确报错，不假装成功。

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rc4"
	"encoding/binary"
	"errors"
	"regexp"
	"strconv"
)

// pdfPasswordPadding 是 PDF 规范里固定的 32 字节填充串（算法 2）。
var pdfPasswordPadding = []byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41, 0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80, 0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}

var (
	encryptRefRe = regexp.MustCompile(`/Encrypt\s+(\d+)\s+(\d+)\s+R`)
	fileIDRe     = regexp.MustCompile(`/ID\s*\[\s*<([0-9A-Fa-f\s]+)>`)
)

type pdfDecrypter struct {
	key  []byte
	aes  bool // true = AESV2(AES-128-CBC)，false = RC4
	obj  int
	skip int // 该对象号是 /Encrypt 本身，它的字符串不加密
}

// buildDecrypter 从 PDF 字节里找到 /Encrypt 与 /ID，按空用户密码推导文件密钥。
// 返回 (nil, nil) 表示文件没有加密。
func buildDecrypter(data []byte) (*pdfDecrypter, error) {
	m := encryptRefRe.FindSubmatch(data)
	if m == nil {
		return nil, nil
	}
	encNum, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return nil, nil
	}
	encDict := findObjectDict(data, encNum)
	if encDict == nil {
		return nil, nil
	}
	if filter := nameOf(encDict["Filter"]); filter != "" && filter != "Standard" {
		return nil, ErrPDFEncrypted
	}
	v := intOf(encDict["V"])
	r := intOf(encDict["R"])
	if r >= 5 || v >= 5 {
		return nil, errors.New("PDF 使用了 AES-256 加密（R5/R6），当前不支持；请用阅读器另存一份再导入")
	}
	lengthBits := intOf(encDict["Length"])
	if lengthBits == 0 {
		lengthBits = 40
	}
	keyLen := lengthBits / 8
	if r == 2 || keyLen == 0 {
		keyLen = 5
	}
	if keyLen > 16 {
		keyLen = 16
	}

	o := []byte(stringOf(encDict["O"]))
	u := []byte(stringOf(encDict["U"]))
	if len(o) < 32 || len(u) < 32 {
		return nil, ErrPDFEncrypted
	}
	id0 := []byte{}
	if idm := fileIDRe.FindSubmatch(data); idm != nil {
		id0 = hexToBytes(string(idm[1]))
	}

	p := int64(int32(intOf(encDict["P"])))
	// /EncryptMetadata false 时（R>=4），密钥推导要多拼 4 个 0xFF（规范算法 2 第 f 步）
	metaStr := true
	if v, ok := encDict["EncryptMetadata"]; ok {
		metaStr = boolOf(v)
	}
	key := pdfFileKey(o, p, id0, r, keyLen, metaStr)

	if !verifyUserPassword(key, o, u, id0, r) {
		// 空密码验不过去 = 真的有密码
		return nil, ErrPDFEncrypted
	}

	dec := &pdfDecrypter{key: key, obj: encNum, skip: encNum}
	if v == 4 {
		dec.aes = cryptFilterIsAES(encDict)
	}
	return dec, nil
}

// pdfFileKey 是规范里的算法 2：由空用户密码 + /O + /P + /ID[0] 推导文件密钥。
func pdfFileKey(o []byte, p int64, id0 []byte, r, keyLen int, encryptMetadata bool) []byte {
	h := md5.New()
	h.Write(pdfPasswordPadding) // 空密码 = 直接用填充串
	h.Write(o[:32])
	var pBytes [4]byte
	binary.LittleEndian.PutUint32(pBytes[:], uint32(uint32(p)))
	h.Write(pBytes[:])
	h.Write(id0)
	if r >= 4 && !encryptMetadata {
		h.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	}
	sum := h.Sum(nil)
	if r >= 3 {
		for i := 0; i < 50; i++ {
			next := md5.Sum(sum[:keyLen])
			sum = next[:]
		}
	}
	return sum[:keyLen]
}

// verifyUserPassword 是规范里的算法 4/5：用空密码算一遍 /U，与文件里的 /U 比对。
func verifyUserPassword(key, o, u, id0 []byte, r int) bool {
	if r == 2 {
		out, err := rc4Bytes(key, pdfPasswordPadding)
		if err != nil {
			return false
		}
		return bytes.Equal(out, u[:32])
	}
	h := md5.New()
	h.Write(pdfPasswordPadding)
	h.Write(id0)
	sum := h.Sum(nil)
	out, err := rc4Bytes(key, sum)
	if err != nil {
		return false
	}
	cur := out
	for i := 1; i <= 19; i++ {
		xorKey := make([]byte, len(key))
		for j := range key {
			xorKey[j] = key[j] ^ byte(i)
		}
		next, err := rc4Bytes(xorKey, cur)
		if err != nil {
			return false
		}
		cur = next
	}
	return bytes.Equal(cur[:16], u[:16])
}

func rc4Bytes(key, data []byte) ([]byte, error) {
	c, err := rc4.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data))
	c.XORKeyStream(out, data)
	return out, nil
}

// cryptFilterIsAES 判断 /V 4 的文档用的是不是 AESV2 加密流。
func cryptFilterIsAES(encDict pdfDict) bool {
	stmf := nameOf(encDict["StmF"])
	cf := encDict["CF"]
	// /StmF /Identity 表示流不加密（少见但合法）
	if stmf == "Identity" {
		return false
	}
	apply := func(name string, entry any) bool {
		dd := dictValue(entry)
		if dd == nil {
			return false
		}
		if name == "Identity" {
			return false
		}
		cfm := nameOf(dd["CFM"])
		if cfm == "" {
			cfm = nameOf(dd["CF"])
		}
		return cfm == "AESV2"
	}
	if dict, ok := cf.(pdfDict); ok {
		if stmf != "" {
			return apply(stmf, dict[stmf])
		}
		if std, ok := dict["StdCF"]; ok {
			return apply("StdCF", std)
		}
	}
	return false
}

func dictValue(v any) pdfDict {
	if dd, ok := v.(pdfDict); ok {
		return dd
	}
	return nil
}

// objectKey 是规范里的算法 1：文件密钥 + 对象号 + 世代号（AES 还要加 "sAlT"）。
func (d *pdfDecrypter) objectKey(objNum, gen int) []byte {
	h := md5.New()
	h.Write(d.key)
	h.Write([]byte{byte(objNum), byte(objNum >> 8), byte(objNum >> 16), byte(gen), byte(gen >> 8)})
	if d.aes {
		h.Write([]byte{0x73, 0x41, 0x6C, 0x54}) // "sAlT"
	}
	sum := h.Sum(nil)
	n := len(d.key) + 5
	if n > 16 {
		n = 16
	}
	return sum[:n]
}

// decrypt 解密一个对象的流数据或字符串。
func (d *pdfDecrypter) decrypt(objNum, gen int, data []byte) []byte {
	if d == nil || len(data) == 0 || objNum == d.skip {
		return data
	}
	key := d.objectKey(objNum, gen)
	if d.aes {
		if len(data) <= aes.BlockSize {
			return nil
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil
		}
		iv := data[:aes.BlockSize]
		body := data[aes.BlockSize:]
		if len(body)%aes.BlockSize != 0 {
			body = body[:len(body)-len(body)%aes.BlockSize]
		}
		out := make([]byte, len(body))
		cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, body)
		return pkcs5Unpad(out)
	}
	out, err := rc4Bytes(key, data)
	if err != nil {
		return data
	}
	return out
}

func pkcs5Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	pad := int(data[len(data)-1])
	if pad <= 0 || pad > 16 || pad > len(data) {
		return data
	}
	for _, b := range data[len(data)-pad:] {
		if int(b) != pad {
			return data
		}
	}
	return data[:len(data)-pad]
}

// findObjectDict 在原始字节里定位 `N G obj ... endobj` 并解析出该对象的字典。
func findObjectDict(data []byte, num int) pdfDict {
	needle := []byte(strconv.Itoa(num) + " 0 obj")
	idx := bytes.Index(data, needle)
	if idx < 0 {
		return nil
	}
	sc := &pdfScanner{buf: data, pos: idx + len(needle)}
	obj, err := sc.parseObject()
	if err != nil {
		return nil
	}
	if dict, ok := obj.(pdfDict); ok {
		return dict
	}
	return nil
}

func hexToBytes(s string) []byte {
	clean := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isPDFWhite(c) {
			continue
		}
		clean = append(clean, c)
	}
	if len(clean)%2 == 1 {
		clean = append(clean, '0')
	}
	out := make([]byte, 0, len(clean)/2)
	for i := 0; i+1 < len(clean); i += 2 {
		b, err := strconv.ParseUint(string(clean[i:i+2]), 16, 8)
		if err != nil {
			return out
		}
		out = append(out, byte(b))
	}
	return out
}

// decryptObjectStrings 解密对象里的字符串（流里已经没有字符串了，只有顶层对象还需要）。
func decryptObjectStrings(obj any, dec *pdfDecrypter, objNum, gen int) any {
	if dec == nil {
		return obj
	}
	switch t := obj.(type) {
	case pdfString:
		out := dec.decrypt(objNum, gen, []byte(t))
		if out == nil {
			return t
		}
		return pdfString(out)
	case pdfDict:
		for k, v := range t {
			t[k] = decryptObjectStrings(v, dec, objNum, gen)
		}
		return t
	case pdfArray:
		for i, v := range t {
			t[i] = decryptObjectStrings(v, dec, objNum, gen)
		}
		return t
	}
	return obj
}
