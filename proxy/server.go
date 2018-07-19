package proxy

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/naiba/proxyinabox"
	"github.com/naiba/proxyinabox/service"
)

var proxyService proxyinabox.ProxyService

//Serv serv the http proxy
func Serv(httpPort, httpsPort string) {
	//init service
	proxyService = &service.ProxyService{DB: proxyinabox.DB}

	//start http proxy server
	httpServer := newServer(httpPort)
	go httpServer.ListenAndServe()
	fmt.Println("HTTP proxy: `http://localhost:" + httpPort + "`")

	//start https proxy server
	var pemPath = "./server.pem"
	var keyPath = "./server.key"
	httpsServer := newServer(httpsPort)
	go httpsServer.ListenAndServeTLS(pemPath, keyPath)
	fmt.Println("HTTPS proxy: `https://localhost:" + httpsPort + "`")
}

func newServer(port string) *http.Server {
	return &http.Server{
		Addr:    ":" + port,
		Handler: http.HandlerFunc(proxyHandler),
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	//simple AntiRobot
	if r.Header.Get("Naiba") != "lifelonglearning" {
		http.Error(w, "Naive", http.StatusForbidden)
		return
	}
	r.Header.Del("Naiba")
	//get user IP
	var ip string
	ipSlice := strings.Split(r.RemoteAddr, ":")
	if len(ipSlice) == 2 {
		ip = ipSlice[0]
	} else {
		ip = "Unknown"
	}
	//check request limit
	if !proxyinabox.CacheInstance.CheckIPLimit(ip) {
		fmt.Println("[x] ip request limited", ip)
		http.Error(w, "The request exceeds the limit, and up to "+fmt.Sprintf("%d", proxyinabox.Config.Sys.RequestLimitPerIP)+" requests at one minute per IP.["+ip+"]", http.StatusForbidden)
		return
	}
	//check domain limit
	var domain = r.URL.Hostname()
	if !proxyinabox.CacheInstance.CheckIPDomain(ip, domain) {
		fmt.Println("[x] ip domain limited", ip)
		http.Error(w, "The request exceeds the limit, and up to "+strconv.Itoa(proxyinabox.Config.Sys.DomainsPerIP)+" domain names are crawled every half hour per IP.["+ip+"]", http.StatusForbidden)
		return
	}
	//set response header
	w.Header().Add("X-Powered-By", "Naiba")

	// dispatch proxy
	p, err := proxyinabox.CacheInstance.GetFreshProxy(domain)
	if err != nil {
		http.Error(w, "Unknown error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Println(domain, "-->", p)

	if r.Method == http.MethodConnect {
		handleTunneling(p, w, r)
	} else {
		handleHTTP(p, w, r)
	}
}
