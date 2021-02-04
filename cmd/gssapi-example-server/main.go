package main

/*
#cgo LDFLAGS: -lgssapi_krb5
#include <stdio.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <gssapi.h>
// 这个宏不能在go中调用，转为函数
int gss_error(OM_uint32 v) {
  return GSS_ERROR(v);
};
*/
import "C"
import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

var listen string
var keytab string

func main() {
	flag.StringVar(&listen, "l", "", "listen addr")
	flag.StringVar(&keytab, "k", "", "keytab file path")
	flag.Parse()
	if len(listen) == 0 || len(keytab) == 0 {
		flag.PrintDefaults()
		return
	}
	// 配置密钥表到环境变量，gss lib会用到
	os.Setenv("KRB5_KTNAME", keytab)
	l, err := net.Listen("tcp", listen)
	if err != nil {
		log.Panic(err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Panic(err)
		}
		go func() {
			defer conn.Close()
			defer func() {
				err := recover()
				if err != nil {
					log.Println("server error", err)
				}
			}()
			s(conn)
		}()
	}
}

func s(conn io.ReadWriter) {
	// 初始化参数，初始化可能发生多次，需要用到上次的值
	var ctxMinor C.OM_uint32
	var outputToken C.gss_buffer_desc
	var clientName C.gss_name_t
	var mechType C.gss_OID
	var retFlags C.OM_uint32
	var ctx C.gss_ctx_id_t = C.GSS_C_NO_CONTEXT
	defer C.gss_delete_sec_context(nil, &ctx, nil)
	for {
		// 从对方那里接收token
		var l uint32
		err := binary.Read(conn, binary.LittleEndian, &l)
		if err != nil {
			log.Panic(err)
		}
		var buff bytes.Buffer
		_, err = io.CopyN(&buff, conn, int64(l))
		if err != nil {
			log.Panic(err)
		}
		inputToken := bytesToBuffer(buff.Bytes())
		defer C.gss_release_buffer(nil, inputToken)
		// 接受上下文
		ctxMajor := C.gss_accept_sec_context(
			&ctxMinor, &ctx, C.GSS_C_NO_CREDENTIAL,
			inputToken, nil, &clientName, &mechType,
			&outputToken, &retFlags, nil, nil)
		if err := gssError(ctxMajor, ctxMinor); err != nil {
			log.Println(err)
		}
		// 有输出token，发送给对方
		if outputToken.length > 0 {
			data := bufferToBytes(&outputToken)
			l = uint32(len(data))
			binary.Write(conn, binary.LittleEndian, &l)
			_, err = conn.Write(data)
			if err != nil {
				log.Panic(err)
			}
		}
		switch ctxMajor {
		// 认证还要继续
		case C.GSS_S_CONTINUE_NEEDED:
			log.Println("GSS_S_CONTINUE_NEEDED", ctxMajor)
		// 认证完成
		case C.GSS_S_COMPLETE:
			log.Println("GSS_S_COMPLETE", ctxMajor)
			var nameBuffer C.gss_buffer_desc
			var minor C.OM_uint32
			major := C.gss_display_name(&minor, clientName, &nameBuffer, nil)
			if err := gssError(major, minor); err != nil {
				log.Panic(err)
			}
			log.Printf("connect: %s", bufferToBytes(&nameBuffer))
			return
		}
	}
}

// 转换GSS error为Golang error
func gssError(majorStatus, minorStatus C.OM_uint32) error {
	if C.gss_error(majorStatus) == 0 {
		return nil
	}
	var minor C.OM_uint32
	var messageContext C.OM_uint32
	var buffer C.gss_buffer_desc
	var err error
	for {
		C.gss_display_status(&minor, majorStatus, C.GSS_C_GSS_CODE, C.GSS_C_NO_OID, &messageContext, &buffer)
		if messageContext == 0 {
			err = fmt.Errorf("major_error(%d): %s", majorStatus, bufferToBytes(&buffer))
			C.gss_release_buffer(nil, &buffer)
			break
		}
	}
	for {
		log.Println(minorStatus)
		C.gss_display_status(&minor, minorStatus, C.GSS_C_MECH_CODE, C.GSS_C_NO_OID, &messageContext, &buffer)
		if messageContext == 0 {
			err = fmt.Errorf(`minor_error(%d): %s; %w`, minorStatus, bufferToBytes(&buffer), err)
			C.gss_release_buffer(nil, &buffer)
			break
		}
	}
	return err
}

// 转换gss buffer为go bytes
func bufferToBytes(buffer *C.gss_buffer_desc) []byte {
	return C.GoBytes(buffer.value, C.int(buffer.length))
}

// 转换go bytes为gss buffer
func bytesToBuffer(data []byte) *C.gss_buffer_desc {
	var buffer C.gss_buffer_desc
	buffer.value = C.CBytes(data)
	buffer.length = C.ulong(len(data))
	return &buffer
}
