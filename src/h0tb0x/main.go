package main

import (
	"code.google.com/p/gopass"
	"encoding/json"
	"flag"
	"fmt"
	"h0tb0x/api"
	"h0tb0x/base"
	"h0tb0x/conn"
	"h0tb0x/crypto"
	"h0tb0x/data"
	"h0tb0x/db"
	"h0tb0x/link"
	"h0tb0x/meta"
	"h0tb0x/nat"
	"h0tb0x/rendezvous"
	"h0tb0x/sync"
	"h0tb0x/transfer"
	"log"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path"
	gosync "sync"
	"time"
)

const (
	PortMapLifetime   = 7200 // 2 hours
	DefaultApiPort    = 8000
	DefaultLinkPort   = 31337 // Should allow 0 to be automatic
	DefaultExtHost    = ""    // Automatic
	DefaultExtPort    = 0     // Automatic
	DefaultRendezvous = "rs.h0tb0x.net:2134"
	DefaultDir        = ".h0tb0x"
	ConfigFilename    = "config.json"
	DbFilename        = "h0tb0x.db"
	RendezvousDb      = "rendezvous.db"
	IdFilename        = "identity"
)

type Config struct {
	ApiPort    uint16 // Port for user API calls, must be set
	LinkPort   uint16 // Port of other h0tb0x's to talk to, 0 *should* means pick randomly, doesn't work yet
	ExtHost    string // External host (for hand forwarding), Empty means use nat-pmp
	ExtPort    uint16 // External port (for hand forwarding), 0 means use nat-pmp
	Rendezvous string // Rendezvous server to use
}

// Flag variables
var useUPnP bool
var useNATPMP bool

// NAT
//var gateway string
var portflag int
var port uint16

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

func newH0tb0x(dir string) {
	cfgFilename := path.Join(dir, ConfigFilename)
	dbFilename := path.Join(dir, DbFilename)
	idFilename := path.Join(dir, IdFilename)

	fmt.Println("Making a *NEW* h0tb0x directory")
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		fatal("", err)
	}
	pass1, err := gopass.GetPass("Please enter the new password for your h0tb0x: ")
	if err != nil {
		os.RemoveAll(dir)
		fatal("", err)
	}
	pass2, err := gopass.GetPass("Re-enter your password: ")
	if err != nil {
		os.RemoveAll(dir)
		fatal("", err)
	}
	if pass1 != pass2 {
		fmt.Println("Passwords don't match, go away")
		os.RemoveAll(dir)
		return
	}
	fmt.Printf("Generating default config, you may want to check %s to make sure values are correct\n", cfgFilename)

	config := &Config{
		ApiPort:    DefaultApiPort,
		LinkPort:   DefaultLinkPort,
		ExtHost:    DefaultExtHost,
		ExtPort:    DefaultExtPort,
		Rendezvous: DefaultRendezvous,
	}
	configFile, err := os.Create(cfgFilename)
	if err != nil {
		os.RemoveAll(dir)
		fatal("", err)
	}
	enc := json.NewEncoder(configFile)
	err = enc.Encode(&config)
	if err != nil {
		os.RemoveAll(dir)
		fatal("", err)
	}
	configFile.Close()
	thedb := db.NewDatabase(dbFilename, "h0tb0x")
	thedb.Close()
	ident := crypto.NewSecretIdentity(pass1)
	identFile, err := os.Create(idFilename)
	if err != nil {
		os.RemoveAll(dir)
		fatal("", err)
	}
	_, err = identFile.Write(transfer.AsBytes(ident.Lock()))
	if err != nil {
		os.RemoveAll(dir)
		fatal("", err)
	}
	identFile.Close()
}

// Top function - will call UPnP or NAT-PMP GetExternalAddr function dependent on parameters
func GetExternalAddrCommon(port uint16) (net.IP, uint16, error) {

	var natobj nat.NAT
	var listenPort int
	var external net.IP

	str, err := GetGateway()
	fmt.Printf("Gateway Address: %q\n", str)

	gatewayIP := net.ParseIP(str)
	if gatewayIP == nil {
		return nil, 0, fmt.Errorf("Invalid gateway:  ", gatewayIP)
	}

	// fmt.Printf("Getting External Address (Common Function)\n")

	// Try NATPMP First
	if !useUPnP {
		log.Println("Attempting to use NATPMP to set up port forwarding.")
		natobj = nat.NewNatPMP(gatewayIP)

		if natobj != nil {
			external, err = natobj.GetExternalAddress()
			if err != nil { // Error Returned
				log.Println("Error:  Unable to get external IP address from router using NATPMP")
				natobj = nil // If there is an error, nil out natobj so we know to try UPnP instead
				// return nil, 0, fmt.Errorf("Error - Unable to get external IP address from router using NATPMP")
			} else {
				log.Println("Router's external IP address discovered: ", external)
			}
		} else { // We don't appear to ever hit this case - natobj is never returned nil from NewNatPMP even if there is an error.
			log.Println("Unable to discover router capabilities using NATPMP", err)
		}
	}

	// If NATPMP failed, or -useUPnP is set, try UPnP
	if !useNATPMP && natobj == nil {
		log.Println("Attempting to use UPnP to set up port forwarding.")
		natobj, err = nat.Discover()
		if err != nil {
			log.Println("Unable to discover router capabilities using UPnP", err)
			//			return nil, 0, fmt.Errorf("Unable to discover router capabilities using UPnP:", err)
		}

		external, err = natobj.GetExternalAddress()
		if err != nil { // Error
			log.Println("Error:  Unable to get external IP address from router using UPnP")
			// return nil, 0, fmt.Errorf("Error - Unable to get external IP address from router using UPnP")
		} else {
			log.Println("Router's external IP address discovered: ", external)
		}
	}

	// If IP discovery failed
	if natobj == nil {
		listenPort = int(port)
		log.Println("WARNING:  Router port forwarding setup failed - peers will be unable to connect through firewall.")
	} else {
		// Request that the router forward the port
		if listenPort, err = nat.ChooseListenPort(natobj, port); err != nil {
			log.Println("Error during router port forwarding request:  ", err)
			log.Println("Peers will be unable to connect through firewall.")
		} else {
			log.Println("Router port forwarding request successful.")
		}
	}

	// Convert port value back to uint16
	port = uint16(listenPort)
	fmt.Printf("Listening on external port %v\n", listenPort)
	return external, port, nil
}

// *** MAIN ***
func main() {
	connMgr := conn.NewNetConnMgr()
	user, err := user.Current()
	if err != nil {
		fatal("Current user is invalid", err)
	}

	defaultDir := path.Join(user.HomeDir, DefaultDir)

	rendezvousPort := flag.Int("r", 0, "Set the rendezvous port and run a rendezvous server instead of h0tb0x")
	dir := flag.String("d", defaultDir, "The directory your h0tb0x stuff lives in")
	flag.BoolVar(&useUPnP, "useUPnP", false, "Only use UPnP to set up router port forwarding (skip NATPMP detection)")
	flag.BoolVar(&useNATPMP, "useNATPMP", false, "Only use NATPMP to set up router port forwarding (skip UPnP detection)")
	flag.Parse()

	if *dir == "" {
		fatal("Directory option is required", nil)
	}
	if *rendezvousPort != 0 {
		rdbFilename := path.Join(*dir, RendezvousDb)
		port := uint16(*rendezvousPort)
		rendezvous.Serve(connMgr, port, rdbFilename)
		return
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
		thedb = db.NewDatabase(dbFilename, "h0tb0x")
	} else {
		fmt.Printf("h0tb0x directory %s doesn't exist\n", *dir)
		newH0tb0x(*dir)
		fmt.Printf("Config created, now you can rerun h0tb0x!\n")
		os.Exit(1)
	}
	fmt.Printf("Running with config: \n")
	fmt.Printf("  ApiPort: %d\n", config.ApiPort)
	fmt.Printf("  LinkPort: %d\n", config.LinkPort)
	fmt.Printf("  Rendezvous: %s\n", config.Rendezvous)
	fmt.Printf("  ExtHost: %s\n", config.ExtHost)
	fmt.Printf("  ExtPort: %d\n", config.ExtPort)

	var extHost net.IP
	var extPort uint16
	if config.ExtHost == "" || config.ExtPort == 0 {
		// fmt.Printf("Getting External Address\n")
		extHost, extPort, err = GetExternalAddrCommon(config.LinkPort)
		if err != nil {
			panic(err)
		}
	} else {
		extHost = net.ParseIP(config.ExtHost)
		if extHost == nil {
			panic(fmt.Errorf("Host part of external host is invalid: %s", config.ExtHost))
		}
		extPort = config.ExtPort
		if extPort == 0 {
			panic(fmt.Errorf("Port part of external host is invalid: %s", err))
		}
	}

	base := &base.Base{
		Log:   log.New(os.Stderr, "", log.LstdFlags),
		Db:    thedb,
		Ident: ident,
		Port:  config.LinkPort,
	}

	link := link.NewLinkMgr(base, connMgr)
	sync := sync.NewSyncMgr(link)
	meta := meta.NewMetaMgr(sync)
	data := data.NewDataMgr(dataDir, meta)
	api := api.NewApiMgr(config.Rendezvous, config.ApiPort, data, connMgr)
	api.SetExt(extHost, extPort)

	stopTime := make(chan bool)
	var stopWait gosync.WaitGroup

	if config.ExtHost == "" {
		stopWait.Add(1)
		go func() {
			for {
				tchan := time.After(15 * time.Minute)
				select {
				case <-stopTime:
					stopWait.Done()
					return
				case <-tchan:
					break
				}
				extHost, extPort, err = GetExternalAddrCommon(config.LinkPort)
				if err != nil {
					log.Printf("GetExternalAddrCommon failed: %v", err)
					continue
				}
				api.SetExt(extHost, extPort)
			}
		}()
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)
	api.Start()
	<-ch
	fmt.Fprintf(os.Stderr, "\n")
	api.Log.Printf("Shutting down")
	api.Log.Printf("Stopping timer")
	close(stopTime)
	stopWait.Wait()
	api.Log.Printf("Timer stopped")
	api.Stop()
}
