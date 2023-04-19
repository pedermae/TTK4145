package config

import (
	"workspace/elevator"
	"fmt"
	"workspace/elevio"
	"strings"
	"strconv"
)

const NumFloors = 4
const DoorOpenDuration = 3
const TRAVEL_TIME = 10
const WatchDogTime = 5
const NumButtons = 3

type Order struct{
	ButtonEvent elevio.ButtonEvent
	State elevator.OrderState
}

type OrderMsg struct{
	Order Order
	Recipient string 
	Sender string
}

type DataMapMessage struct {
	Id string
	Elevators map[string]elevator.Elevator
}

func DeepCopyDataMapMessage(dataMapMessage DataMapMessage) DataMapMessage {
    copiedElevators := make(map[string]elevator.Elevator)
    for id, elev := range dataMapMessage.Elevators {
        copiedElevator, err := elevator.DeepCopyElevator(elev)
		if err != nil{
			fmt.Println(err)
		}
        copiedElevators[id] = copiedElevator
    }
    newDataMapMessage := DataMapMessage{
        Id:        dataMapMessage.Id,
        Elevators: copiedElevators,
    }
    return newDataMapMessage
}

func (dm DataMapMessage) PrintDataMapMessage(){
	for _, elevator := range dm.Elevators{
		fmt.Println("Elevator: ", elevator.Id)
		elevator.PrintElevator()
	}
}

func(dm OrderMsg) PrintOrderMsg(){
	fmt.Println("OrderMsg from: ", dm.Sender, " to: ", dm.Recipient)
	fmt.Print("Containing order: ", dm.Order)
}

func LowestID(localdatamap DataMapMessage)bool{
	minimum_id := 9999999999
	localElevator_ID := localdatamap.Id
	for id, e := range localdatamap.Elevators{
		if e.CurrentState != elevator.Unavailable{
			last_part := strings.Split(id, "-")[2]
			num_id, _ := strconv.Atoi(last_part)
			if num_id < minimum_id{
				minimum_id = num_id
			}
		}
	}
	
	local_num_id_string := strings.Split(localElevator_ID, "-")[2]
	local_num_id, _ := strconv.Atoi(local_num_id_string)

	if local_num_id == minimum_id{
		return true
	}else{
		return false
	}
}