package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"time"
)

const (
	usage                       = `usage: wiresteward (server|agent)`
	defaultServerPeerConfigPath = "servers.json"
	defaultLeaserSyncInterval   = 1 * time.Minute
	defaultLeaseTime            = 12 * time.Hour
	defaultLeasesFilename       = "/etc/wiresteward/leases"
)

var (
	wireguardServerPeerConfigPath = os.Getenv("WGS_SERVER_PEER_CONFIG_PATH")
	serverConfig                  map[string]string // static config that the server will pass to potential peers
	userPeerSubnet                *net.IPNet
	leaserSyncInterval            time.Duration
	ipLeaseTime                   = os.Getenv("WGS_IP_LEASE_TIME")
	leasesFilename                = os.Getenv("WGS_IP_LEASEs_FILENAME")
	flagSet                       = flag.NewFlagSet("", flag.ExitOnError)
	flagConfig                    = flagSet.String("config", "", "(Required for agent) Path of the config file")
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalln(usage)
	}
	switch os.Args[1] {
	case "server":
		server()
	case "agent":
		agent()
	default:
		log.Fatalln(usage)
	}
}

// reads config into serverConfig map[string]string
func readServerStaticConfig() {
	if wireguardServerPeerConfigPath == "" {
		wireguardServerPeerConfigPath = defaultServerPeerConfigPath
	}
	sc, err := ioutil.ReadFile(wireguardServerPeerConfigPath)
	if err != nil {
		log.Fatalf("Could not load server peer info: %v", err)
	}
	if err := json.Unmarshal(sc, &serverConfig); err != nil {
		log.Fatalf("Could not parse server peer info: %v", err)
	}
	if _, ok := serverConfig["AllowedIPs"]; !ok {
		log.Fatal("server static config missing `AllowedIPs`")
	}
	if _, ok := serverConfig["Endpoint"]; !ok {
		log.Fatal("server static config missing `Endpoint`")
	}
}

func server() {
	if leaserSyncInterval == 0 {
		leaserSyncInterval = defaultLeaserSyncInterval
	}
	leasetime := defaultLeaseTime
	var err error
	if ipLeaseTime != "" {
		leasetime, err = time.ParseDuration(ipLeaseTime)
		if err != nil {
			log.Fatal(err)
		}
	}
	if leasesFilename == "" {
		leasesFilename = defaultLeasesFilename
	}

	ups := os.Getenv("WGS_USER_PEER_SUBNET")
	if ups == "" {
		log.Fatal("Environment variable WGS_USER_PEER_SUBNET is not set")
	}
	_, network, err := net.ParseCIDR(ups)
	if err != nil {
		log.Fatalf("Could not parse user peer subnet: %v", err)
	}
	if err := initWithFile(leasesFilename, network, leasetime); err != nil {
		log.Fatalf("Cannot start lease server: %v", err)
	}

	// Read the static config that server will provide to peers
	readServerStaticConfig()

	go newLeaseHandler()
	ticker := time.NewTicker(leaserSyncInterval)
	defer ticker.Stop()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	log.Print("Starting leaser loop")
	for {
		select {
		case <-ticker.C:
			if err := syncWgRecords(); err != nil {
				log.Print(err)
			}
		case <-quit:
			log.Print("Quitting")
			return
		}
	}
}

func getAgentConfigPathFromHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Cannot get user's home dir to read config: %v", err)
	}
	return path.Join(home, "wiresteward.json")
}

func agent() {

	flagSet.Parse(os.Args[2:])

	cfgPath := *flagConfig
	if cfgPath == "" {
		cfgPath = getAgentConfigPathFromHome()
		log.Printf(
			"no -config flag found, will try default path: %s\n",
			cfgPath,
		)
	}

	agentConf, err := ReadAgentConfig(cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	tokenHandler := NewOauthTokenHandler(
		agentConf.Oidc.AuthUrl,
		agentConf.Oidc.TokenUrl,
		agentConf.Oidc.ClientID,
	)

	for _, dev := range *agentConf.Devs {
		// Create an agent for each dev specified in the config
		agent, err := NewAgent(
			dev.Name,
			tokenHandler,
		)
		if err != nil {
			log.Fatalf(
				"Cannot create agent fot dev: %s : %v",
				dev.Name,
				err,
			)
		}

		// Clear all the device ips, new ones will be added according
		// to peers responses
		if err := agent.FlushDeviceIPs(); err != nil {
			log.Fatalf(
				"Cannot clear ips fot dev: %s : %v",
				dev.Name,
				err,
			)
		}

		for _, peer := range *dev.Peers {
			if err := agent.GetNewWgLease(peer.Url); err != nil {
				log.Printf(
					"cannot get lease from peer: %s :%v",
					peer.Url,
					err,
				)
			}
		}
	}
}
