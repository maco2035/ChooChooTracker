package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/jamespfennell/gtfs"
)

// Define a struct to represent real-time transit updates.
type RealTimeUpdate struct {
	RouteID     string `json:"route_id"`
	StopID      string `json:"stop_id"`
	ArrivalTime int64  `json:"arrival_time"`
	// Add other relevant fields as needed.
}

func main() {
	// Read configuration from config.json file.
	config, err := readConfig("config.json")
	if err != nil {
		fmt.Println("Error reading config file:", err)
		return
	}

	// Create a new Discord session using the provided bot token.
	discord, err := discordgo.New("Bot " + config.Discord_token)
	if err != nil {
		fmt.Println("Error creating Discord session:", err)
		return
	}
	// Register the messageCreate func as a callback for MessageCreate events.
	discord.AddHandler(messageCreate)
	discord.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAll)
	// Open a websocket connection to Discord and begin listening.
	err = discord.Open()
	if err != nil {
		fmt.Println("Error opening connection:", err)
		return
	}

	fmt.Println("Bot is now running. Press CTRL+C to exit.")

	// Wait here until CTRL+C or other term signal is received.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	// Cleanly close down the Discord session.
	discord.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages sent by the bot itself.
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if the message starts with the command prefix.
	switch {
	case strings.HasPrefix(m.Content, "!schedule"):
		// Split the message into parts to extract the query parameters.
		parts := strings.Fields(m.Content)

		// Check if the command has at least 2 parts.
		if len(parts) < 2 {
			s.ChannelMessageSend(m.ChannelID, "Usage: !schedule <agency_id>")
			return
		}

		agencyID := parts[1] // Extract the agency ID from the message.

		responseStatic, err := http.Get("https://www.bart.gov/dev/schedules/google_transit.zip")
		staicBytes, err := io.ReadAll(responseStatic.Body)
		staticData, err := gtfs.ParseStatic(staicBytes, gtfs.ParseStaticOptions{})
		if staticData != nil {
			fmt.Println("Static Data doesn't exists")
		}
		// Fetch real-time GTFS data.
		resp, err := http.Get("http://api.bart.gov/gtfsrt/tripupdate.aspx")
		realtimeBytes, err := io.ReadAll(resp.Body)
		realtimeData, err := gtfs.ParseRealtime(realtimeBytes, &gtfs.ParseRealtimeOptions{})

		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Error fetching GTFS data: "+err.Error())
			return
		}

		// Match trip IDs and get current locations of trains.
		trainsLocations := make(map[string]string) // Map to store train locations.
		for _, tripUpdate := range realtimeData.Trips {
			// Find the corresponding trip in the static data.
			tripID := tripUpdate.ID.ID
			// Get the current stop for the trip from the trip update.

			var currentStop string
			if tripUpdate.Vehicle.StopID != nil {
				currentStop = *tripUpdate.Vehicle.StopID // Dereference the pointer to get the string value.

			}

			// Add the train's current location to the map.
			trainsLocations[tripID] = currentStop
		}

		// Check if there are any routes found.
		if len(realtimeData.Trips) == 0 {
			s.ChannelMessageSend(m.ChannelID, "No routes running on bart.")
			return
		}
		var response strings.Builder
		response.WriteString(fmt.Sprintf("Current train locations for agency %s:\n", agencyID))
		for tripID, stopID := range trainsLocations {
			message := fmt.Sprintf("Train %s is currently at stop %s\n", tripID, stopID)
			if len(response.String())+len(message) > 1900 {
				s.ChannelMessageSend(m.ChannelID, response.String())
				response.Reset()
			}
			response.WriteString(message)
		}

		s.ChannelMessageSend(m.ChannelID, response.String())
		if err != nil {
			fmt.Println("Error sending message to guild channel:", err)
			return
		}

	case strings.HasPrefix(m.Content, "!trains"):
		// Split the message into parts to extract the query parameters.
		parts := strings.Fields(m.Content)

		// Check if the command has at least 2 parts.
		if len(parts) < 2 {
			s.ChannelMessageSend(m.ChannelID, "Usage: !trains <agency_id>")
			return
		}

		resp, _ := http.Get("http://api.bart.gov/gtfsrt/tripupdate.aspx")
		b, _ := io.ReadAll(resp.Body)
		realtimeData, err := gtfs.ParseRealtime(b, &gtfs.ParseRealtimeOptions{})
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Error fetching GTFS data: "+err.Error())
			return
		}
		fmt.Printf("The SF BART currently has %d trains running or scheduled\n", len(realtimeData.Trips))

		agencyID := parts[1] // Extract the agency ID from the message.
		// if agencyID != realtimeData.Agency.name {
		// 	s.ChannelMessageSend(m.ChannelID, "Usage: !trains <agency_id>")
		// 	return
		// }

		// Check if the train is currently on a trip.
		currentTrips := numberOfTrainsOnTrip(realtimeData)

		var response strings.Builder
		response.WriteString("The current number of trains for " + agencyID + ":\n")
		// Send the response message to the user.
		if currentTrips > 0 {
			response.WriteString(fmt.Sprintf("There are %s trains is currently on %d trips.", agencyID, currentTrips))
		} else {
			response.WriteString(fmt.Sprintf("Train %s is not currently on any trips.", agencyID))
		}
		_, err = s.ChannelMessageSend(m.ChannelID, response.String())

	}
}

func numberOfTrainsOnTrip(realtimeData *gtfs.Realtime) int {
	currentTrips := 0
	for _, trip := range realtimeData.Trips {
		if v := trip.GetVehicle(); v == *trip.Vehicle {
			currentTrips++
		}
	}
	return currentTrips
}
