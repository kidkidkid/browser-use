package controller

import "fmt"

type Action struct {
	Name        string
	Description string
	Params      any
}

var actions = make(map[string]*Action)

func RegistryAction(name string, description string, parmas any) {
	if actions[name] != nil {
		panic(fmt.Sprint("%s already resgistered", name))
	}
	actions[name] = &Action{
		Name:        name,
		Description: description,
		Params:      parmas,
	}
}
