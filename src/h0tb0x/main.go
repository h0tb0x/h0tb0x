package main

import (
	"code.google.com/p/go-nat-pmp"
	"code.google.com/p/gopass"
	"encoding/json"
	"flag"
	"fmt"
	"h0tb0x/api"
	"h0tb0x/base"
	"h0tb0x/crypto"
	"h0tb0x/data"
	"h0tb0x/db"
	"h0tb0x/link"
	"h0tb0x/meta"
	"h0tb0x/sync"
	"h0tb0x/transfer"
	"log"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path"
	"strings"
	"strconv"
	gosync "sync"
	"time"
)

const (
	PortMapLifetime = 7200 // 2 hours
	DefaultApiPort  = 8000
	DefaultLinkPort = 31337
	DefaultDir      = ".h0tb0x"
	ConfigFilename  = "config.json"
	DbFilename      = "h0tb0x.db"
	IdFilename      = "identity"
)

type Config struct {
	ApiPort  uint16
	LinkPort uint16
}

func GetExternalAddr(port uint16) (net.IP, uint16) {
	str, err := GetGateway()
	fmt.Printf("Gateway Address: %q\n", str)

	gateway := net.ParseIP(str)
	if gateway == nil {
		panic(fmt.Errorf("Invalid gateway"))
	}

	nat := natpmp.NewClient(gateway)
	extaddr, err := nat.GetExternalAddress()
	if err != nil {
		panic(err)
	}
	ip := net.IPv4(
		extaddr.ExternalIPAddress[0],
		extaddr.ExternalIPAddress[1],
		extaddr.ExternalIPAddress[2],
		extaddr.ExternalIPAddress[3],
	)
	fmt.Printf("External Address: %v\n", ip)

	res, err := nat.AddPortMapping("tcp", int(port), 0, PortMapLifetime)
	if err != nil {
		panic(err)
	}

	fmt.Printf("External Port: %v\n", res.MappedExternalPort)
	return ip, res.MappedExternalPort
}

func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, msg)
	if err != nil {
		if msg != "" {
			fmt.Fprintf(os.Stderr, ":")
		}
		fmt.Fprintf(os.Stderr, "%v", err)
	}
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

func main() {
	user, err := user.Current()
	if err != nil {
		fatal("Current user is invalid", err)
	}

	defaultDir := path.Join(user.HomeDir, DefaultDir)

	dir := flag.String("d", defaultDir, "The directory your h0tb0x stuff lives in")
	extHostStr := flag.String("h", "", "The external host (default chosen by nat-pmp)")
	flag.Parse()
	if *dir == "" {
		fatal("Directory option is required", nil)
	}

	cfgFilename := path.Join(*dir, ConfigFilename)
	dbFilename := path.Join(*dir, DbFilename)
	idFilename := path.Join(*dir, IdFilename)
	dataDir := path.Join(*dir, "data")

	var config *Config
	var thedb *db.Database
	var ident *crypto.SecretIdentity
	if fi, err := os.Stat(*dir); err == nil && fi.IsDir() {
		pass1, err := gopass.GetPass("Please enter your h0tb0x password: ")
		if err != nil {
			fatal("", err)
		}
		configFile, err := os.Open(cfgFilename)
		if err != nil {
			fatal("", err)
		}
		dec := json.NewDecoder(configFile)
		err = dec.Decode(&config)
		if err != nil {
			fatal("", err)
		}
		configFile.Close()
		identFile, err := os.Open(idFilename)
		if err != nil {
			fatal("", err)
		}
		var lockedId *crypto.LockedIdentity
		err = transfer.Decode(identFile, &lockedId)
		if err != nil {
			fatal("", err)
		}
		ident, err = crypto.UnlockSecretIdentity(lockedId, pass1)
		if err != nil {
			fatal("", err)
		}
		thedb = db.NewDatabase(dbFilename)
	} else {
		fmt.Println("Making a *NEW* h0tb0x config")
		err = os.MkdirAll(*dir, 0700)
		if err != nil {
			fatal("", err)
		}
		pass1, err := gopass.GetPass("Please enter the new password for your h0tb0x: ")
		if err != nil {
			fatal("", err)
		}
		pass2, err := gopass.GetPass("Re-enter your password: ")
		if err != nil {
			fatal("", err)
		}
		if pass1 != pass2 {
			fmt.Println("Passwords don't match, go away")
			return
		}
		config = &Config{
			ApiPort:  DefaultApiPort,
			LinkPort: DefaultLinkPort,
		}
		configFile, err := os.Create(cfgFilename)
		if err != nil {
			fatal("", err)
		}
		enc := json.NewEncoder(configFile)
		err = enc.Encode(&config)
		if err != nil {
			fatal("", err)
		}
		configFile.Close()
		thedb = db.NewDatabase(dbFilename)
		thedb.Install()
		ident = crypto.NewSecretIdentity(pass1)
		identFile, err := os.Create(idFilename)
		if err != nil {
			fatal("", err)
		}
		_, err = identFile.Write(transfer.AsBytes(ident.Lock()))
		if err != nil {
			fatal("", err)
		}
	}

	var extHost net.IP
	var extPort uint16
	if *extHostStr == "" {
		extHost, extPort = GetExternalAddr(config.LinkPort)
	} else {
		parts := strings.Split(*extHostStr, ":")
		if len(parts) != 2 {
			panic(fmt.Errorf("Unable to parse host string, no ':' seperator", *extHostStr))
		}
		extHost = net.ParseIP(parts[0]) 
		if extHost == nil {
			panic(fmt.Errorf("Host part of external host is invalid: %s", parts[0]))
		}
		extPortInt, err := strconv.Atoi(parts[1])
		if extPortInt < 1 || extPortInt > 65535 {
			panic(fmt.Errorf("External port out of range"))
		}
		extPort = uint16(extPortInt)
		if err != nil || extPort == 0 {
			panic(fmt.Errorf("Port part of external host is invalid: %s", err))
		}
	}

	base := &base.Base{
		Log:   log.New(os.Stderr, "h0tb0x", log.LstdFlags),
		Db:    thedb,
		Ident: ident,
		Port:  config.LinkPort,
	}

	link := link.NewLinkMgr(base)
	sync := sync.NewSyncMgr(link)
	meta := meta.NewMetaMgr(sync)
	data := data.NewDataMgr(dataDir, meta)
	api := api.NewApiMgr(extHost.String(), extPort, config.ApiPort, data)

	stopTime := make(chan bool)
	var stopWait gosync.WaitGroup

	if *extHostStr == "" {
		stopWait.Add(1)
		go func() {
			for {
				tchan := time.After(15 * time.Minute)
        			select {
        				case <- stopTime:
						stopWait.Done()
                				return
        				case <-tchan:
					break
                		}
				api.SetExt(GetExternalAddr(config.LinkPort))
			}
		}()
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)
	api.Run()
	<-ch
	fmt.Fprintf(os.Stderr, "\n")
	api.Log.Printf("Shutting down")
	api.Log.Printf("Stopping timer")
	close(stopTime)
	stopWait.Wait()
	api.Log.Printf("Timer stopped")
	api.Stop()
}
