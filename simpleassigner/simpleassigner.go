package simpleassigner

import (
	"workspace/config"
	"workspace/elevio"
	"workspace/simpleassigner/cost"
	"workspace/elevator"
)

type Tuple struct{
	Id string
	Time int
}

func AssignOrder(datamap config.DataMapMessage, btnevent elevio.ButtonEvent) (string, bool){
	//If someone is currently doing the call we wont send another elevator to the same floor
	should_be_served := true
	datamapcopy := config.DeepCopyDataMapMessage(datamap)

	var elevator_times[]Tuple

	for id, elev := range datamapcopy.Elevators{
		if elev.OrderQueue[btnevent.Floor][btnevent.Button] == elevator.ConfirmedOrder{
			should_be_served = false
		}
		elevator_times = append(elevator_times, Tuple{id, cost.Cost(elev, btnevent)})
	}
	
	min_id := "mpty"
	min_time := 99999

	for i := 0; i < len(elevator_times); i++ {
		if elevator_times[i].Time < min_time{ 
			min_time = elevator_times[i].Time
			min_id = elevator_times[i].Id
		}
	}
	if min_id == "mpty"{
		should_be_served = false
	}
	return min_id, should_be_served
}

