package luna

import (
	"fmt"
	"math/rand"
)

// Names collection
var names = []string{
	"John",
	"Paul",
	"George",
	"Ringo",
	"Michael",
	"Eric",
	"Kurt",
	"Dave",
	"James",
	"Robert",
	"Ozzy",
	"Kirk",
	"Jimi",
	"Janis",
}

// SurNames collection
var surnames = []string{
	"Smith",
	"Johnson",
	"Williams",
	"Brown",
	"Jones",
	"Miller",
	"Davis",
	"Garcia",
	"Rodriguez",
	"Wilson",
	"Martinez",
	"Anderson",
	"Taylor",
	"Thomas",
	"Hernandez",
	"Moore",
	"Martin",
	"Jackson",
	"Thompson",
	"White",
	"Lopez",
	"Lee",
	"Gonzalez",
	"Harris",
	"Clark",
	"Lewis",
	"Robinson",
	"Walker",
	"Perez",
	"Hall",
}

// Generate random user name
func randomUserName() string {
	return fmt.Sprintf("%s %s", names[rand.Intn(len(names))], surnames[rand.Intn(len(surnames))])
}
