package main

import (
	"fmt"
	"os"
	"time"
	"net/http"
	"os/signal"
	"io/ioutil"
	"strings"
	"log"
	"syscall"
	"image"
	"github.com/bwmarrin/discordgo"
	"github.com/corona10/goimagehash"
	"bufio"
	"gopkg.in/yaml.v2"
)

var pokemonNames []string
var currentPokemon string
var currentChannelID string
var pokemonReceived chan string = make(chan string, 1)

type Hashes struct {
	Hashes map[string]string
}

func main() {
	fmt.Println("Welcome to the hash updater. To update the hashes type in any channel with the desired pokemon bot: hasher-update")
	client, err := discordgo.New("token")
	if (err != nil) {
		fmt.Printf("Invalid token or session error: %s", err)
		os.Exit(1)
	}

	err = client.Open()
	if err != nil {
		fmt.Println("Error opening session")
		os.Exit(1)
	}
	client.AddHandler(messageCreate)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	client.Close()
}

func messageCreate(client *discordgo.Session, message *discordgo.MessageCreate) {
	if (!message.Author.Bot && strings.HasPrefix(message.Content, "hasher-")) {
		command := strings.TrimPrefix(message.Content, "hasher-")
		switch command {
		case "update":
			HashUpdater(client, message.ChannelID)
		case "info":
			client.ChannelMessageSend(message.ChannelID, "This hasher bot has been created by 0xSteeW. https://github.com/0xSteeW")
		}

	} else if message.Author.Bot && len(message.Embeds) != 0 && message.ChannelID == currentChannelID && currentPokemon != "" {
		fmt.Printf("Received image for pokemon: %s\n", currentPokemon)
		url := message.Embeds[0].Image.URL
		imageDecoder := Download(url)
		hash := Hash(imageDecoder)
		pokemonReceived <- hash
	}
}

// File functions
func WriteList(content []byte) {
	err := ioutil.WriteFile("updated.yaml", content, 0644)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Could not write hashes to yaml file.")
	}
}

//
func ReadNames() []string {
	var names []string
	ioReader, err := os.Open("./pokemons.txt")
	defer ioReader.Close()
	if err != nil {
		fmt.Println("Could not read pokemon names file.")
		os.Exit(1)
	}
	scanner := bufio.NewScanner(ioReader)
	for scanner.Scan() {
		names = append(names, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return names
	}
	return names
}

func HashUpdater(client *discordgo.Session, channelID string) {
	currentChannelID = channelID
	hashes := new(Hashes)
	hashes.Hashes = make(map[string]string)
	pokemonNames = ReadNames()
	for idx, name := range pokemonNames {
		time.Sleep(3*time.Second)
		currentPokemon = name
		fmt.Printf("Hasing now: %s, %d/%d", name, idx, len(pokemonNames))
		client.ChannelMessageSend(channelID, fmt.Sprintf("p!info %s", name))
		hash := <- pokemonReceived
		hashes.Hashes[name] = hash
	}
	// For loop finished, marshal the hashes into YAML format
	content, err := yaml.Marshal(hashes)
	if err != nil {
		fmt.Println("Could not parse into YAML")
		os.Exit(1)
	}
	WriteList(content)
}

func Download(url string) *image.Image {
	response, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer response.Body.Close()
	// Body is a io ReadCloser, so we can pass it to image.Decode, which receives an io.Reader
	decoded, _, err := image.Decode(response.Body)
	return &decoded
}

func Hash(imageDecoder *image.Image) string {
	hash, err := goimagehash.PerceptionHash(*imageDecoder)
	if err != nil {
		log.Panic("Could not get the hash of ", currentPokemon)
	}
	return hash.ToString()
}
