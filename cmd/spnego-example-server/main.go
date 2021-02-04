package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jcmturner/goidentity"
	"github.com/jcmturner/gokrb5/v8/keytab"
	"github.com/jcmturner/gokrb5/v8/spnego"
)

const KEY = "creds"

var listen string
var keytabFile string

func main() {
	flag.StringVar(&listen, "l", "", "listen addr")
	flag.StringVar(&keytabFile, "k", "", "keytab file path")
	flag.Parse()
	if len(listen) == 0 || len(keytabFile) == 0 {
		flag.PrintDefaults()
		return
	}

	engine := gin.Default()
	engine.Use(authSpnego(keytabFile))
	engine.GET("/", func(c *gin.Context) {
		creds := c.MustGet(KEY).(goidentity.Identity)
		c.JSON(200, gin.H{
			"username":   creds.UserName(),
			"name":       creds.DisplayName(),
			"session_id": creds.SessionID(),
			"auth_at":    creds.AuthTime(),
			"attr":       creds.Attributes(),
		})
	})
	engine.Run(listen)
}

func authSpnego(keytabFile string) gin.HandlerFunc {
	kt := keytab.New()
	b, err := ioutil.ReadFile(keytabFile)
	if err != nil {
		log.Println(err)
	}
	err = kt.Unmarshal(b)
	if err != nil {
		log.Println(err)
	}
	return func(c *gin.Context) {
		h := func(rw http.ResponseWriter, r *http.Request) {
			creds := goidentity.FromHTTPRequestContext(r)
			c.Set(KEY, creds)
		}
		spnego.SPNEGOKRB5Authenticate(http.HandlerFunc(h), kt).ServeHTTP(c.Writer, c.Request)
		if _, ok := c.Get(KEY); !ok {
			c.Abort()
		}
		return
	}
}
