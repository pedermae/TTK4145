package elevator

import (
	"fmt"
	"workspace/elevio"
	"encoding/json"
)

type State int
const (
	Idle      State = 0
	Door_Open State = 1
	Moving    State = 2
	Unavailable State = 3
)

type OrderState int
const (
	NoOrder        OrderState = 0
	ActiveOrder    OrderState = 1
	ConfirmedOrder OrderState = 2
	FinishedOrder  OrderState = 3
)

type Elevator struct {
	CurrentFloor 	int
	Direction 		elevio.MotorDirection //The direction the elevator is currently / last was moving. 
	CurrentState 	State
	OrderQueue [][]	OrderState
	Id         		string
}

func InsertOrder(buttonEvent elevio.ButtonEvent, state OrderState, elevator Elevator) [][]OrderState {
	if state == ConfirmedOrder {
		elevio.SetButtonLamp(buttonEvent.Button, buttonEvent.Floor, true)
	}
	tmpqueue := elevator.OrderQueue
	tmpqueue[buttonEvent.Floor][buttonEvent.Button] = state
	return tmpqueue
}

func InitElevator(currentFloor int, direction elevio.MotorDirection, state State, floors int, Id string, port string, shouldBind bool) Elevator {
	orderQueue := make([][]OrderState, floors)
	for floor := range orderQueue {
		orderQueue[floor] = make([]OrderState, 3)
	}

	elevator := Elevator{currentFloor, direction, Idle, orderQueue, Id}

	if shouldBind {
		elevio.Init("localhost:"+port, 4)
		elevio.SetFloorIndicator(currentFloor)
		elevio.SetDoorOpenLamp(false)
	
		for floor := range orderQueue {
			for btn := elevio.ButtonType(0); btn < 3; btn++ {
				elevio.SetButtonLamp(btn, floor, false)
			}
		}
	}
	return elevator
}

func (elev Elevator) OrdersAbove() bool {
	for floor := elev.CurrentFloor + 1; floor < 4; floor++ {
		for btn := range elev.OrderQueue[floor] {
			if elev.OrderQueue[floor][btn] == ConfirmedOrder {
				return true
			}
		}
	}
	return false
}

func (elev Elevator) OrdersBelow() bool {
	for floor := 0; floor < elev.CurrentFloor; floor++ {
		for btn := range elev.OrderQueue[floor] {
			if elev.OrderQueue[floor][btn] == ConfirmedOrder {
				return true
			}
		}
	}
	return false
}

func DecideDirection(elev Elevator) elevio.MotorDirection{
	switch elev.Direction {
	case elevio.MD_Up:
		if elev.OrdersAbove() {
			return elevio.MD_Up

		} else if elev.OrdersBelow() {
			return elevio.MD_Down

		} else {
			return elevio.MD_Stop
		}
	
	case elevio.MD_Down:
		fallthrough

	case elevio.MD_Stop:
		if elev.OrdersBelow() {
			return elevio.MD_Down

		} else if elev.OrdersAbove() {
			return elevio.MD_Up

		} else {
			return elevio.MD_Stop
		}
	}
	return elevio.MD_Stop
}

func (elev Elevator) ShouldStop() bool {
	switch {
	case elev.Direction == elevio.MD_Down:

		return elev.OrderQueue[elev.CurrentFloor][elevio.BT_HallDown] == ConfirmedOrder ||
			elev.OrderQueue[elev.CurrentFloor][elevio.BT_Cab] == ConfirmedOrder ||
			!elev.OrdersBelow()

	case elev.Direction == elevio.MD_Up:
		return elev.OrderQueue[elev.CurrentFloor][elevio.BT_HallUp] == ConfirmedOrder ||
			elev.OrderQueue[elev.CurrentFloor][elevio.BT_Cab] == ConfirmedOrder ||
			!elev.OrdersAbove()

	default:
		return true
	}
}

func DeepCopyElevator(e Elevator) (Elevator, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return Elevator{}, err
	}
	var newElevator Elevator
	err = json.Unmarshal(b, &newElevator)
	if err != nil {
		return Elevator{}, err
	}
	return newElevator, nil
}

var OrderStates = map[OrderState]string{
	NoOrder:        "-",
	ActiveOrder:    "A",
	ConfirmedOrder: "C",
	FinishedOrder:  "F",
}

var Directions = map[int]string{
	-1: "DOWN",
	0:  "IDLE/STOP",
	1:  "UP",
}

var States = map[int]string{
	0: "idle",
	1: "door_open",
	2: "moving",
	3: "unavailable",
}

func (e Elevator) PrintElevator() {
	fmt.Println("Current floor: ", e.CurrentFloor)
	fmt.Println("Direction: ", Directions[int(e.Direction)])
	fmt.Println("State: ", States[int(e.CurrentState)])
	fmt.Println("Floor | Hall_Up | Hall_Down | Cab       |")
	for floor := range e.OrderQueue {
		fmt.Print("   ", floor, "  | ")
		for button := range e.OrderQueue[floor] {
			fmt.Print("    ", OrderStates[(e.OrderQueue[floor][button])], "   |   ")
		}
		fmt.Println("")
	}
	fmt.Println("")
}