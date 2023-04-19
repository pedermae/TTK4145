package cost

import (
	"fmt"
	"workspace/config"
	"workspace/elevator"
	"workspace/elevio"
)



func Cost(elev elevator.Elevator, btnevent elevio.ButtonEvent) int {
	simelev, err := elevator.DeepCopyElevator(elev)
	if err != nil {
		fmt.Println("Error in cost.go: Cost()", err)
		return 999999999999
	}
	if simelev.CurrentState == elevator.Unavailable {
		return 999999999
	}

	duration := 0

	simelev.OrderQueue[btnevent.Floor][btnevent.Button] = elevator.ConfirmedOrder

	switch elev.CurrentState {
	case elevator.Idle:
		dir := elevator.DecideDirection(simelev)
		simelev.Direction = dir
		if dir == elevio.MD_Stop {
			return duration
		}

	case elevator.Moving:
		duration += config.TRAVEL_TIME / 2
		elev.CurrentFloor += int(elev.Direction)
	case elevator.Door_Open:
		duration -= config.DoorOpenDuration / 2
	}

	for {
		if simelev.CurrentFloor < 0 || simelev.CurrentFloor >= config.NumFloors {
			return duration
		}
		if simelev.ShouldStop() {
			simelev := clearOrderAtFloor(simelev)
			duration += config.DoorOpenDuration
			dir := elevator.DecideDirection(simelev)
			if dir == elevio.MD_Stop {
				return duration
			}
			simelev.Direction = dir
		}
		simelev.CurrentFloor += int(simelev.Direction)
		duration += config.TRAVEL_TIME
	}
}

func clearOrderAtFloor(simelev elevator.Elevator) elevator.Elevator {
	simelev.OrderQueue[simelev.CurrentFloor][int(elevio.BT_Cab)] = elevator.NoOrder
	switch {
	case simelev.Direction == elevio.MD_Up:
		simelev.OrderQueue[simelev.CurrentFloor][int(elevio.BT_HallUp)] = elevator.NoOrder
		if !simelev.OrdersAbove() {
			simelev.OrderQueue[simelev.CurrentFloor][int(elevio.BT_HallDown)] = elevator.NoOrder
		}
	case simelev.Direction == elevio.MD_Down:
		simelev.OrderQueue[simelev.CurrentFloor][int(elevio.BT_HallDown)] = elevator.NoOrder
		if !simelev.OrdersBelow() {
			simelev.OrderQueue[simelev.CurrentFloor][int(elevio.BT_HallUp)] = elevator.NoOrder
		}
	}
	return simelev
}
