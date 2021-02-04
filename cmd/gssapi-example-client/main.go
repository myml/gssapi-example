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
)

var server string
var name string

func main() {
	flag.StringVar(&server, "s", "", "server addr")
	flag.StringVar(&name, "n", "", "name")
	flag.Parse()
	if len(server) == 0 || len(name) == 0 {
		flag.PrintDefaults()
		return
	}
	conn, err := net.Dial("tcp", server)
	if err != nil {
		log.Panic(err)
	}
	defer conn.Close()
	c(conn)
}

func c(conn io.ReadWriter) {
	// 导入服务名
	var target C.gss_name_t = C.GSS_C_NO_NAME
	defer C.gss_release_name(nil, &target)
	var major, minor C.OM_uint32
	var buffer = bytesToBuffer([]byte(name))
	major = C.gss_import_name(&minor, buffer, C.GSS_C_NT_HOSTBASED_SERVICE, &target)
	if err := gssError(major, minor); err != nil {
		log.Panic("get name", err)
	}
	// 初始化参数，初始化可能会多次运行，并且需要用到上次的参数
	var ctxMinor C.OM_uint32
	var ctx C.gss_ctx_id_t = C.GSS_C_NO_CONTEXT
	defer C.gss_delete_sec_context(nil, &ctx, nil)
	var inputToken C.gss_buffer_desc
	var refFlags C.OM_uint32
	for {
		log.Println("initting")
		var outputToken C.gss_buffer_desc
		// 初始化上下文
		var major = C.gss_init_sec_context(&ctxMinor, C.GSS_C_NO_CREDENTIAL, &ctx,
			target, C.GSS_C_NO_OID, C.GSS_C_MUTUAL_FLAG, 0,
			nil, &inputToken, nil, &outputToken,
			&refFlags, nil)
		if err := gssError(major, ctxMinor); err != nil {
			log.Panic(err)
		}
		// 有输出token，发送给对方
		if outputToken.length > 0 {
			var out = bufferToBytes(&outputToken)
			C.gss_release_buffer(nil, &outputToken)
			C.gss_release_buffer(nil, &inputToken)
			err := binary.Write(conn, binary.LittleEndian, uint32(len(out)))
			if err != nil {
				panic(err)
			}
			_, err = conn.Write(out)
			if err != nil {
				panic(err)
			}
		}
		switch major {
		// 需要继续认证，接收对方的参数，并重新初始化上下文
		case C.GSS_S_CONTINUE_NEEDED:
			log.Println("GSS_S_CONTINUE_NEEDED", outputToken.length)
			var l uint32
			err := binary.Read(conn, binary.LittleEndian, &l)
			if err != nil {
				panic(err)
			}
			var buff bytes.Buffer
			_, err = io.CopyN(&buff, conn, int64(l))
			if err != nil {
				panic(err)
			}
			inputToken = *bytesToBuffer(buff.Bytes())
		// 认证完成
		case C.GSS_S_COMPLETE:
			log.Println("GSS_S_COMPLETE")
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
