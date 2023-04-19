package distributor

import (
	"fmt"
	"time"
	"workspace/config"
	"workspace/elevator"
	"workspace/elevio"
	"workspace/network/peers"
	"workspace/simpleassigner"
)

func Distributor(peerUpdateCh chan peers.PeerUpdate,
	sendTX chan elevator.Elevator,
	receiveTX chan elevator.Elevator,
	localElevator elevator.Elevator,
	elevChan chan elevator.Elevator,
	buttonEventChan chan elevio.ButtonEvent,
	orderChan chan config.Order,
	completedChan chan config.Order,
	sendOrderTX chan config.OrderMsg,
	receiveOrderTX chan config.OrderMsg,
	petWatchDogChan chan bool,
	watchDogStuckChan chan bool) {

	//Initialize the local datamap to the local elevator
	localdatamap := config.DataMapMessage{Elevators: make(map[string]elevator.Elevator)}
	localdatamap.Id = localElevator.Id
	localdatamap.Elevators[localElevator.Id] = localElevator
	

	timerChan := time.After(15*time.Second)

	broadcastTimerChan := time.After(100*time.Millisecond)	//Timer used to broadcast the local elevator's states to other nodes

	redelegateCabTimer := time.NewTimer(10*time.Second)
	redelegateCabTimer.Stop()

	redelegateHallordrTimer := time.NewTimer(10*time.Second)
	redelegateHallordrTimer.Stop()

	//Block incoming cab orders
	blockIncomingCabOrderTimer := time.After(4*time.Second)
	blockIncomingCabOrder := false

	var cab_revival []string
	var hall_revival []string


	for{		
		select{
			case p := <- peerUpdateCh:
				fmt.Printf("Peer update:\n")
				fmt.Printf("  Peers:    %q\n", p.Peers)
				fmt.Printf("  New:      %q\n", p.New)
				fmt.Printf("  Lost:     %q\n", p.Lost)

				
				if len(p.New) > 0  && p.New != localElevator.Id{
					_, ok := localdatamap.Elevators[p.New]
					if ok && config.LowestID(localdatamap) {
						redelegateCabTimer.Reset(2*time.Second)
						cab_revival = append(cab_revival, p.New)
			
					}else if !(ok) {	
						localdatamap.Elevators[p.New] = elevator.InitElevator(0, elevio.MD_Stop, elevator.Idle, config.NumFloors, p.New, "", false)
					}
					break
				}
				if len(p.Lost) > 0{
					//Update the lost elevators to unavailable
					localdatamap = updatePeersLost(localdatamap, p.Lost)

					if config.LowestID(localdatamap){	
						hall_revival = append(hall_revival, p.Lost...)
						redelegateHallordrTimer.Reset(1*time.Second)
					}
				}
			case <- blockIncomingCabOrderTimer:
				blockIncomingCabOrder = true	//When this happen's I won't receive my cab-calls back after a period of loss of internet

			case <- redelegateCabTimer.C:
				fmt.Println(cab_revival)
				for _, el := range cab_revival{
					redelegateCaborders(el, localdatamap, sendOrderTX)
				}
				cab_revival = nil
			
			case <- redelegateHallordrTimer.C:
				for i := range hall_revival{
					localdatamap = reAssignHallOrders(localdatamap, hall_revival[i], sendOrderTX, orderChan)
				}
				hall_revival = nil

				
			case buttonEvent := <- buttonEventChan:
				if buttonEvent.Button == elevio.BT_Cab{
					order := config.Order{ButtonEvent: buttonEvent, State: elevator.ConfirmedOrder}
					ordermsg := config.OrderMsg{Order: order, Sender: localElevator.Id, Recipient: localElevator.Id}
					localdatamap.Elevators[localElevator.Id].OrderQueue[buttonEvent.Floor][buttonEvent.Button] = elevator.ConfirmedOrder
					orderChan <- order
					broadcast(sendOrderTX, ordermsg)

				}else{
					recipient, should_be_served := simpleassigner.AssignOrder(localdatamap, buttonEvent)
					if should_be_served{
						order := config.Order{ButtonEvent: buttonEvent, State: elevator.ConfirmedOrder}
						if recipient == localElevator.Id{
							orderChan <- order
						}
						localdatamap.Elevators[recipient].OrderQueue[buttonEvent.Floor][buttonEvent.Button] = elevator.ConfirmedOrder
						ordermsg := config.OrderMsg{Order: order, Sender: localElevator.Id, Recipient: recipient}
						broadcast(sendOrderTX, ordermsg)
					}
				}
	
			case message_elev := <- receiveTX:
				if message_elev.Id == localdatamap.Id{
					break	//We don't service echo messages to ourselves
				}
				if  ourhim, ok := localdatamap.Elevators[message_elev.Id]; ok{
					ourhim.CurrentState = message_elev.CurrentState
					ourhim.CurrentFloor = message_elev.CurrentFloor
					ourhim.Direction = message_elev.Direction
					localdatamap.Elevators[message_elev.Id] = ourhim
				}

			case update := <- elevChan:
				petWatchDogChan <- false
				localElevator,_ = elevator.DeepCopyElevator(update)
				localdatamap.Elevators[localElevator.Id] = localElevator


			case <- watchDogStuckChan:
				if localElevator.CurrentState == elevator.Idle{
					petWatchDogChan <- false
				} else{
					localElevator.CurrentState = elevator.Unavailable
					localdatamap.Elevators[localElevator.Id] = localElevator
					localdatamap = reAssignHallOrders(localdatamap, localElevator.Id, sendOrderTX, orderChan)
				}

			case completed := <- completedChan:
				completedmsg := config.OrderMsg{Order: completed, Sender: localElevator.Id, Recipient: "all"}
				broadcast(sendOrderTX, completedmsg)
			
			case rec := <- receiveOrderTX:
				if rec.Order.ButtonEvent.Button != elevio.BT_Cab && rec.Order.State == elevator.ConfirmedOrder{
					elevio.SetButtonLamp(rec.Order.ButtonEvent.Button, rec.Order.ButtonEvent.Floor, true)
				}
				if rec.Sender != localElevator.Id{
					if rec.Recipient != localElevator.Id && rec.Order.State == elevator.ConfirmedOrder{
						_, ok := localdatamap.Elevators[rec.Sender]
						if ok{
							localdatamap.Elevators[rec.Recipient].OrderQueue[rec.Order.ButtonEvent.Floor][rec.Order.ButtonEvent.Button] = rec.Order.State
							break
						}
					}
					if rec.Recipient == "all"{
						_, ok := localdatamap.Elevators[rec.Sender]
						if ok{
							localdatamap.Elevators[rec.Sender].OrderQueue[rec.Order.ButtonEvent.Floor][rec.Order.ButtonEvent.Button] = elevator.NoOrder
						}
						if rec.Order.ButtonEvent.Button != elevio.BT_Cab && rec.Order.State == elevator.FinishedOrder{
							elevio.SetButtonLamp(rec.Order.ButtonEvent.Button, rec.Order.ButtonEvent.Floor, false)
						}
						break
					}
					if rec.Order.State == elevator.ConfirmedOrder && rec.Recipient == localElevator.Id{
						//Possibility to block and resend incoming cab-orders if we have been operative while the network was out
						if rec.Order.ButtonEvent.Button == elevio.BT_Cab && blockIncomingCabOrder{
							//We block and resend the orders as finished so everyone understands that they was completed during loss of internet
							if localElevator.OrderQueue[rec.Order.ButtonEvent.Floor][rec.Order.ButtonEvent.Button] == elevator.NoOrder{
								finished_order := config.Order{ButtonEvent: rec.Order.ButtonEvent, State: elevator.FinishedOrder}
								finished_order_msg := config.OrderMsg{Order: finished_order, Recipient: "all", Sender: localElevator.Id}
								broadcast(sendOrderTX, finished_order_msg)
								break
							}else{
								break
							}
						}
						if localdatamap.Elevators[localElevator.Id].OrderQueue[rec.Order.ButtonEvent.Floor][rec.Order.ButtonEvent.Button] != elevator.ConfirmedOrder{
							localElevator.OrderQueue[rec.Order.ButtonEvent.Floor][rec.Order.ButtonEvent.Button] = elevator.ConfirmedOrder
							localdatamap.Elevators[localElevator.Id] = localElevator
							orderChan <- rec.Order
						}
					}
				}

			case <- broadcastTimerChan:
				copy_elev, _ := elevator.DeepCopyElevator(localElevator)
				sendTX <- copy_elev
				broadcastTimerChan = time.After(time.Millisecond * 100)

			case <- timerChan:
				fmt.Println("---------------I AM: ", localElevator.Id, "------------------------------------")
				localdatamap.PrintDataMapMessage()
				timerChan = time.After(time.Second * 5)
		}	
	}
}

func updatePeersLost(localdatamap config.DataMapMessage, lost []string) config.DataMapMessage{
	for _, lostID := range lost{
		if ourHim, ok := localdatamap.Elevators[lostID]; ok{
			ourHim.CurrentState = elevator.Unavailable
			localdatamap.Elevators[lostID] = ourHim
		}
	}
	return localdatamap
}

func broadcast(sendOrderTX chan config.OrderMsg, msg config.OrderMsg,){
	for i := 0; i < 25; i++ {sendOrderTX <- msg}
}

func redelegateCaborders(revivedElevID string,localdatamap config.DataMapMessage,  sendOrderTX chan config.OrderMsg){
	local_id := localdatamap.Id
    for floor := 0; floor < config.NumFloors; floor++ {
        if localdatamap.Elevators[revivedElevID].OrderQueue[floor][elevio.BT_Cab] == elevator.ConfirmedOrder {
            btnevent := elevio.ButtonEvent{Floor: floor, Button: elevio.ButtonType(elevio.BT_Cab)}
            order := config.Order{ButtonEvent: btnevent, State: elevator.ConfirmedOrder}
            ordermsg := config.OrderMsg{Order: order, Sender: local_id, Recipient: revivedElevID}
            broadcast(sendOrderTX, ordermsg)
        }
    }
}

func reAssignHallOrders(dm config.DataMapMessage, DonorId string, sendOrderTX chan config.OrderMsg, orderChan chan config.Order) config.DataMapMessage {
	if Elevator, ok := dm.Elevators[DonorId]; ok {
		for floor := 0; floor < config.NumFloors; floor++ {
			for button := 0; button < (config.NumButtons - 1); button++ {
				if Elevator.OrderQueue[floor][button] == elevator.ConfirmedOrder {
					Elevator.OrderQueue[floor][button] = elevator.NoOrder
					dm.Elevators[DonorId] = Elevator
					btnevent := elevio.ButtonEvent{Floor: floor, Button: elevio.ButtonType(button)}
					recipient, should_be_served := simpleassigner.AssignOrder(dm, btnevent)

					if should_be_served {

						order := config.Order{ButtonEvent: btnevent, State: elevator.ConfirmedOrder}

						//When we donate away our own orders
						if DonorId == dm.Id {
							ordermsg := config.OrderMsg{Order: order, Sender: Elevator.Id, Recipient: recipient}
							broadcast(sendOrderTX, ordermsg)

							no_order := config.Order{ButtonEvent: btnevent, State: elevator.NoOrder}
							no_order_msg := config.OrderMsg{Order: no_order, Sender: Elevator.Id, Recipient: "all"}
							broadcast(sendOrderTX, no_order_msg)

							local_order := config.Order{ButtonEvent: btnevent, State: elevator.NoOrder}
							orderChan <- local_order

						//When we redelegate donor's orders
						} else {
							Elevator.OrderQueue[floor][button] = elevator.NoOrder
							dm.Elevators[Elevator.Id] = Elevator

							if recipient == dm.Id {
								orderChan <- order
							}
							ordermsg := config.OrderMsg{Order: order, Sender: dm.Id, Recipient: recipient}
							broadcast(sendOrderTX, ordermsg)

							no_order := config.Order{ButtonEvent: btnevent, State: elevator.NoOrder}
							no_order_msg := config.OrderMsg{Order: no_order, Sender: Elevator.Id, Recipient: "all"}
							broadcast(sendOrderTX, no_order_msg)
						}
					}

				}
			}
		}
	}
	return dm
}