package main

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/corona10/goimagehash"
	"github.com/oliamb/cutter"
	"gopkg.in/yaml.v2"
)

var pokemonNames []string
var currentPokemon string
var currentChannelID string
var pokemonReceived chan []string = make(chan []string, 1)

type Hashes struct {
	Hashes map[string][]string
}

func main() {
	fmt.Println("Welcome to the hash updater. To update the hashes type in any channel with the desired pokemon bot: hasher-update")
	client, err := discordgo.New("token")
	if err != nil {
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
	if !message.Author.Bot && strings.HasPrefix(message.Content, "hasher-") {
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
		phash, dhash := Hash(CropUselessArea(imageDecoder))
		hash := []string{phash, dhash}
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
	hashes.Hashes = make(map[string][]string)
	pokemonNames = ReadNames()
	for idx, name := range pokemonNames {
		time.Sleep(3 * time.Second)
		currentPokemon = name
		fmt.Printf("Hasing now: %s, %d/%d", name, idx, len(pokemonNames))
		client.ChannelMessageSend(channelID, fmt.Sprintf("p!dex %s", name))
		hash := <-pokemonReceived
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

func Hash(imageDecoder image.Image) (string, string) {
	phash, err := goimagehash.PerceptionHash(imageDecoder)
	dhash, err := goimagehash.DifferenceHash(imageDecoder)
	if err != nil {
		log.Panic("Could not get the hash of ", currentPokemon)
	}
	return phash.ToString(), dhash.ToString()
}
func CropUselessArea(img *image.Image) image.Image {
	topLeft, bottomRight, transparent := FindVisibleVertexes(*img)
	size := image.Point{X: bottomRight.X - topLeft.X, Y: bottomRight.Y - topLeft.Y}
	fmt.Println(size)
	newImg, _ := cutter.Crop(transparent, cutter.Config{
		Width:  size.X,
		Height: size.Y,
		Anchor: topLeft,
		Mode:   cutter.TopLeft,
	})
	return newImg
}

func FindVisibleVertexes(img image.Image) (image.Point, image.Point, image.Image) {
	var COLOR_TRESHOLD int8 = 50
	// Iterate over img.At(), because it gives a color.Color object. Test if that color.Color is not empty, and seek for the nearest to each border.
	sizeX := img.Bounds().Max.X
	sizeY := img.Bounds().Max.Y

	// Create a new RGBA image, make a copy of img, but remove any pixel with alpha < COLOR_TRESHOLD
	transparent := image.NewRGBA(img.Bounds())

	// First get top left vertex, starting from left border
	fmt.Printf("Size: %d,%d\n", sizeX, sizeY)
	var currentLowest int
	var currentVertex image.Point
	var topLeft image.Point
	var bottomRight image.Point
	// sizeX < sizeY ? sizeY+1 : sizeX+1 , assign whichever is higer, and to the max size of the image, so no value can be higher than currentLowest
	if sizeX < sizeY {
		currentLowest = sizeY + 1
	} else {
		currentLowest = sizeX + 1
	}

	// Left border
	for row := 0; row < sizeY; row++ {
		for pixel := 0; pixel < sizeX; pixel++ {
			c := img.At(pixel, row)
			_, _, _, alpha := c.RGBA()
			if int8(alpha) > COLOR_TRESHOLD {
				transparent.Set(pixel, row, color.Transparent)
				// Found non-transparent pixel, check if the distance from lowest is less
				if pixel < currentLowest {
					currentLowest = pixel
					currentVertex = image.Point{X: pixel, Y: row}
				}
				// Break current column after having found non-transparent pixel
			} else {
				transparent.Set(pixel, row, c)
			}

		}
	}
	topLeft.X = currentVertex.X

	if sizeX < sizeY {
		currentLowest = sizeY + 1
	} else {
		currentLowest = sizeX + 1
	}
	currentVertex = image.Point{0, 0}
	// Top border
	for column := 0; column < sizeX; column++ {
		for pixel := 0; pixel < sizeY; pixel++ {
			c := img.At(column, pixel)
			_, _, _, alpha := c.RGBA()
			if int8(alpha) > COLOR_TRESHOLD {
				transparent.Set(column, pixel, color.Transparent)
				// Found non-transparent pixel, check if the distance from lowest is less
				if pixel < currentLowest {
					currentLowest = pixel
					currentVertex = image.Point{X: column, Y: pixel}
				}
			} else {
				transparent.Set(column, pixel, c)
			}

		}
	}
	topLeft.Y = currentVertex.Y
	if sizeX < sizeY {
		currentLowest = sizeY + 1
	} else {
		currentLowest = sizeX + 1
	}
	// Right
	for row := 0; row < sizeY; row++ {
		// Just change the pixel direction (y stays)
		for pixel := sizeX - 1; pixel >= 0; pixel-- {
			c := img.At(pixel, row)
			_, _, _, alpha := c.RGBA()
			if int8(alpha) > COLOR_TRESHOLD {
				transparent.Set(sizeX-1-pixel, row, color.Transparent)
				// Found non-transparent pixel, check if the distance from lowest is less
				if pixel < currentLowest {
					currentLowest = pixel
					currentVertex = image.Point{X: sizeX - 1 - pixel, Y: row}
				}
				// Break current column after having found non-transparent pixel
			} else {
				transparent.Set(sizeX-1-pixel, row, c)
			}

		}
	}
	bottomRight.X = currentVertex.X
	if sizeX < sizeY {
		currentLowest = sizeY + 1
	} else {
		currentLowest = sizeX + 1
	}
	currentVertex = image.Point{0, 0}

	// Bottom
	for column := 0; column < sizeX; column++ {
		for pixel := sizeY - 1; pixel >= 0; pixel-- {
			c := img.At(column, pixel)
			_, _, _, alpha := c.RGBA()
			if int8(alpha) > COLOR_TRESHOLD {
				transparent.Set(column, sizeY-1-pixel, color.Transparent)
				// Found non-transparent pixel, check if the distance from lowest is less
				if pixel < currentLowest {
					currentLowest = pixel
					currentVertex = image.Point{X: column, Y: sizeY - 1 - pixel}
				}
			} else {
				transparent.Set(column, sizeY-1-pixel, c)
			}

		}
	}
	bottomRight.Y = currentVertex.Y

	return topLeft, bottomRight, transparent
}
