package fsm

import (
	"fmt"
	"time"
	"workspace/config"
	"workspace/elevator"
	"workspace/elevio"
)

func FSM(elev elevator.Elevator, 
	elevChan chan elevator.Elevator,
	orderChan chan config.Order,
	floorChan chan int,
	stopChan chan bool,
	obstrChan chan bool,
	completedChan chan config.Order) {

	//Initialize the elevator to a defined state
	elevio.SetMotorDirection(elevio.MD_Down)
	elev.CurrentState = elevator.Moving
	elev.Direction = elevio.MD_Down
	floor := <-floorChan
	for {
		if floor < config.NumFloors {
			elev.CurrentFloor = floor
			elevio.SetFloorIndicator(floor)
			elevio.SetMotorDirection(elevio.MD_Stop)
			elev.CurrentState = elevator.Idle
			elevio.SetFloorIndicator(elev.CurrentFloor)
			break
		}
	}
	elevChan <- elev
	doorTimer := time.NewTimer(time.Duration(config.DoorOpenDuration)*time.Second)

	// Finite state machine
	for {
		select {
		case order := <-orderChan:
		
			elev.OrderQueue[order.ButtonEvent.Floor][order.ButtonEvent.Button] = order.State

			if order.State == elevator.NoOrder{
				elevio.SetButtonLamp(order.ButtonEvent.Button, order.ButtonEvent.Floor, false)
				break
			}
			
			switch elev.CurrentState {

			case elevator.Moving:
				elevio.SetButtonLamp(order.ButtonEvent.Button, order.ButtonEvent.Floor, true)
				elevChan <- elev

			case elevator.Door_Open:

				if elev.CurrentFloor == order.ButtonEvent.Floor {
					doorTimer.Reset(time.Duration(config.DoorOpenDuration) * time.Second)
					elev.CurrentState = elevator.Door_Open
					elevio.SetButtonLamp(order.ButtonEvent.Button, order.ButtonEvent.Floor, true)
					elevChan <- elev

				} else {
					elevio.SetButtonLamp(order.ButtonEvent.Button, order.ButtonEvent.Floor, true)
					elevChan <- elev
				}

			case elevator.Idle:
				
				if elev.CurrentFloor == order.ButtonEvent.Floor {
					elevio.SetDoorOpenLamp(true)
					elevio.SetButtonLamp(order.ButtonEvent.Button, order.ButtonEvent.Floor, true)
					doorTimer.Reset(time.Duration(config.DoorOpenDuration) * time.Second)
					elev.CurrentState = elevator.Door_Open

					elevChan <- elev
				} else {
					elevio.SetButtonLamp(order.ButtonEvent.Button, order.ButtonEvent.Floor, true)
					newDir := elevator.DecideDirection(elev)
					if newDir == elevio.MD_Stop{
						fmt.Println("This should not happen!! Decided to stop")
					}
					
					elevio.SetMotorDirection(newDir)
					elev.Direction = newDir
					elev.CurrentState = elevator.Moving

					elevChan <- elev
				}
			}
			
		case floor := <-floorChan:
			elev.CurrentFloor = floor
			elevio.SetFloorIndicator(floor)
			switch elev.CurrentState {

			case elevator.Moving:
				if elev.ShouldStop() {
					elev.CurrentState = elevator.Idle
					elevio.SetMotorDirection(elevio.MD_Stop)
			
					elevio.SetDoorOpenLamp(true)
					doorTimer.Reset(time.Duration(config.DoorOpenDuration) * time.Second)
					elev.CurrentState = elevator.Door_Open
				}
				elevChan <- elev
			default:
				elevChan <- elev
			}

		case <-doorTimer.C:
			switch {
			case elev.CurrentState == elevator.Door_Open:
				newqueue, events := ClearOrderAtFloor(elev.Direction, elev)
				elev.OrderQueue = newqueue
				compeleteEvents(events, completedChan)
				
				if DoubleHallCall(elev) {
					doorTimer.Reset(time.Duration(config.DoorOpenDuration)*time.Second)
					elevio.SetDoorOpenLamp(true)
					elev.CurrentState = elevator.Door_Open
					elevChan <- elev
					break
				}

				elevio.SetDoorOpenLamp(false)
				newDir := elevator.DecideDirection(elev)

				if newDir != elevio.MD_Stop{
					elev.CurrentState = elevator.Moving
					elev.Direction = newDir
				}else {
					elev.CurrentState = elevator.Idle
				}
				elevio.SetMotorDirection(newDir)
				elevChan <- elev
			}

		case obstruction := <- obstrChan:
			if elev.CurrentState == elevator.Door_Open && obstruction{
				doorTimer.Reset(time.Duration(config.DoorOpenDuration) * time.Second)
			}
		}
	}	

}

func ClearOrderAtFloor(dir elevio.MotorDirection, elev elevator.Elevator) ([][]elevator.OrderState, []config.Order) {
	deecopy_elev, _ := elevator.DeepCopyElevator(elev)
	tmpqueue := deecopy_elev.OrderQueue
	currentfloor := deecopy_elev.CurrentFloor
	
	var events []config.Order

	switch{
	case dir == elevio.MD_Up:
		if tmpqueue[currentfloor][elevio.BT_Cab] == elevator.ConfirmedOrder{
			tmpqueue[currentfloor][elevio.BT_Cab] = elevator.NoOrder
			btnevent := elevio.ButtonEvent{Floor: currentfloor, Button: elevio.BT_Cab}
			order := config.Order{ButtonEvent: btnevent, State: elevator.FinishedOrder}
			events = append(events, order)
		}
		if tmpqueue[currentfloor][elevio.BT_HallUp] == elevator.ConfirmedOrder{
			tmpqueue[currentfloor][elevio.BT_HallUp] = elevator.NoOrder
			btnevent := elevio.ButtonEvent{Floor: currentfloor, Button: elevio.BT_HallUp}
			order := config.Order{ButtonEvent: btnevent, State: elevator.FinishedOrder}
			events = append(events, order)

		}else if tmpqueue[currentfloor][elevio.BT_HallDown] == elevator.ConfirmedOrder && !elev.OrdersAbove(){
			tmpqueue[currentfloor][elevio.BT_HallDown] = elevator.NoOrder
			btnevent := elevio.ButtonEvent{Floor: currentfloor, Button: elevio.BT_HallDown}
			order := config.Order{ButtonEvent: btnevent, State: elevator.FinishedOrder}
			events = append(events, order)
		}
 	
	case dir == elevio.MD_Down:
		if tmpqueue[currentfloor][elevio.BT_Cab] == elevator.ConfirmedOrder{
			tmpqueue[currentfloor][elevio.BT_Cab] = elevator.NoOrder

			btnevent := elevio.ButtonEvent{Floor: currentfloor, Button: elevio.BT_Cab}
			order := config.Order{ButtonEvent: btnevent, State: elevator.FinishedOrder}
			events = append(events, order)
		}

		if tmpqueue[currentfloor][elevio.BT_HallDown] == elevator.ConfirmedOrder{
			tmpqueue[currentfloor][elevio.BT_HallDown] = elevator.NoOrder
			btnevent := elevio.ButtonEvent{Floor: currentfloor, Button: elevio.BT_HallDown}
			order := config.Order{ButtonEvent: btnevent, State: elevator.FinishedOrder}
			events = append(events, order)

		}else if tmpqueue[currentfloor][elevio.BT_HallUp] == elevator.ConfirmedOrder && !elev.OrdersBelow(){
			tmpqueue[currentfloor][elevio.BT_HallUp] = elevator.NoOrder
			btnevent := elevio.ButtonEvent{Floor: currentfloor, Button: elevio.BT_HallUp}
			order := config.Order{ButtonEvent: btnevent, State: elevator.FinishedOrder}
			events = append(events, order)
		}
	}
	return tmpqueue, events
}

func DoubleHallCall(elev elevator.Elevator) bool{
	switch elev.Direction{
	case elevio.MD_Up:
		if elev.OrderQueue[elev.CurrentFloor][elevio.BT_HallUp] != elevator.ConfirmedOrder && elev.OrderQueue[elev.CurrentFloor][elevio.BT_HallDown] == elevator.ConfirmedOrder && !elev.OrdersAbove(){
			return true
		}else{
			return false
		}
	case elevio.MD_Down:
		if elev.OrderQueue[elev.CurrentFloor][elevio.BT_HallDown] != elevator.ConfirmedOrder && elev.OrderQueue[elev.CurrentFloor][elevio.BT_HallUp] == elevator.ConfirmedOrder && !elev.OrdersBelow(){
			return true
		}else{
			return false
		}
	default:
		return false
	}
}

func compeleteEvents(events []config.Order, completedChan chan config.Order){
	for i := 0; i < len(events); i++ {
		completedChan <- events[i] 
		elevio.SetButtonLamp(events[i].ButtonEvent.Button, events[i].ButtonEvent.Floor, false)
	}
}