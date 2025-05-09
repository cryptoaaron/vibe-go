package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"vibe-ai/internal/network"
	"vibe-ai/internal/wallet"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// loadOrCreateIdentity loads a private key from path, or creates one if not found.
func loadOrCreateIdentity(path string) (crypto.PrivKey, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, generate a new key
			log.Printf("Identity file not found at %s. Generating new key...", path)
			privKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
			if err != nil {
				return nil, fmt.Errorf("failed to generate key pair: %w", err)
			}

			// Marshal the private key
			keyBytes, err := crypto.MarshalPrivateKey(privKey)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal private key: %w", err)
			}

			// Write the key to the file with restricted permissions
			err = ioutil.WriteFile(path, keyBytes, 0600)
			if err != nil {
				return nil, fmt.Errorf("failed to write identity file %s: %w", path, err)
			}
			log.Printf("Generated and saved new identity to %s", path)
			return privKey, nil
		} else {
			// Other error reading file
			return nil, fmt.Errorf("failed to read identity file %s: %w", path, err)
		}
	}

	// File exists, unmarshal the key
	privKey, err := crypto.UnmarshalPrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal private key from %s: %w", path, err)
	}
	log.Printf("Loaded identity from %s", path)
	return privKey, nil
}

func main() {
	listenPort := flag.Int("port", 0, "Port number to listen on (0=random)")
	connectAddr := flag.String("connect", "", "Multiaddress of a peer to connect to initially")
	mine := flag.Bool("mine", false, "Enable mining on this node")
	minerAddr := flag.String("mineraddress", "", "Base58 address to receive mining rewards (required if -mine=true)")
	identityPath := flag.String("identity", "./vibe_node.key", "Path to the node identity key file")
	flag.Parse()

	if *mine && *minerAddr == "" {
		log.Fatal("-mineraddress is required when -mine is enabled")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- Load or Create Identity ---
	privKey, err := loadOrCreateIdentity(*identityPath)
	if err != nil {
		log.Fatalf("Failed to load or create identity: %v", err)
	}

	// --- Blockchain Initialization (moved to network.NewNode) ---
	// bc, err := core.NewBlockchain()
	// ...
	// defer bc.Close()

	// --- Create and Start P2P Node ---
	node, err := network.NewNode(ctx, *listenPort, privKey) // Pass identity key
	if err != nil {
		log.Fatalf("Failed to create network node: %v", err)
	}
	// Defer closing resources
	defer node.Host.Close()
	defer node.Blockchain.Close() // Close blockchain DB on shutdown

	node.Start() // Register handlers and start background tasks

	// --- Start Mining (if enabled) ---
	if *mine {
		minerPubKeyHash, err := wallet.Base58Decode(*minerAddr)
		if err != nil {
			log.Fatalf("Invalid miner address format (expected Base58): %v", err)
		}
		node.StartMining(minerPubKeyHash)
	}

	// Connect to an initial peer if specified
	if *connectAddr != "" {
		err = node.ConnectToPeer(*connectAddr)
		if err != nil {
			log.Printf("Failed to connect to initial peer %s: %v", *connectAddr, err)
		} else {
			// Extract peer ID for block request
			peerMA, _ := multiaddr.NewMultiaddr(*connectAddr)
			peerInfo, _ := peer.AddrInfoFromP2pAddr(peerMA)

			// Get our current latest block hash
			myLastHash, err := node.Blockchain.GetLastBlockHashBytes()
			if err != nil {
				log.Printf("Failed to get own last block hash for sync request: %v", err)
			} else {
				log.Printf("Requesting blocks from peer %s after block %x", peerInfo.ID, myLastHash)
				receivedBlocks, err := node.RequestBlocks(peerInfo.ID, myLastHash)
				if err != nil {
					log.Printf("Error requesting blocks from peer %s: %v", peerInfo.ID, err)
				} else {
					// Process received blocks
					log.Printf("Processing %d received blocks...", len(receivedBlocks))
					for _, block := range receivedBlocks {
						err := node.Blockchain.AddBlock(block) // Use node.Blockchain
						if err != nil {
							log.Printf("Failed to add received block %x (Height: %d): %v", block.Hash, block.Height, err)
							break
						}
					}
				}
			}
		}
	}

	log.Println("Node is running. Press Ctrl+C to exit.")

	// Wait for termination signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	log.Println("Shutting down node...")
}
