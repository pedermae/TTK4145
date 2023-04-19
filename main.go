package main

import (
	"flag"
	"fmt"
	"workspace/config"
	"workspace/distributor"
	"workspace/elevator"
	"workspace/elevio"
	"workspace/fsm"
	"workspace/network/bcast"
	"workspace/network/localip"
	"workspace/network/peers"
	"workspace/watchdog"
)

func main() {
	localIP, err := localip.LocalIP()
	if err != nil {
		fmt.Println(err)
		localIP = "DISCONNECTED"
	}

	var peer_id string
	var id string
	var port string

	flag.StringVar(&id, "id", "99", "id for elevator, used to distinguish between multiple instances og the same program running on one computer")
	flag.StringVar(&port, "port", "15657", "port for connecting to the Sim/Hardware")
	flag.Parse()

	peer_id = fmt.Sprintf("peer-%s-%s", localIP, id)
	fmt.Println(" --- I am : ", peer_id, " -----")

	peerUpdateCh := make(chan peers.PeerUpdate)
	peerTxEnable := make(chan bool)
	go peers.Transmitter(17102, peer_id, peerTxEnable)
	go peers.Receiver(17102, peerUpdateCh)

	elev := elevator.InitElevator(0, elevio.MD_Stop, elevator.Idle, 4, peer_id, port, true)

	elevChan := make(chan elevator.Elevator, 100)

	buttonEventChan := make(chan elevio.ButtonEvent, 100)
	orderChan := make(chan config.Order, 100)
	completedChan := make(chan config.Order, 100)

	floorChan := make(chan int)
	stopChan := make(chan bool)
	obstrChan := make(chan bool)

	petWatchDogChan := make(chan bool)
	watchDogStuckChan := make(chan bool)

	go watchdog.Watchdog(config.WatchDogTime, petWatchDogChan, watchDogStuckChan)

	go elevio.PollButtons(buttonEventChan)
	go elevio.PollFloorSensor(floorChan)
	go elevio.PollStopButton(stopChan)
	go elevio.PollObstructionSwitch(obstrChan)

	go fsm.FSM(elev, elevChan, orderChan, floorChan, stopChan, obstrChan, completedChan)

	sendTX := make(chan elevator.Elevator, 100)
	receiveTX := make(chan elevator.Elevator, 100)

	sendOrderTX := make(chan config.OrderMsg, 100)
	receiveOrderTX := make(chan config.OrderMsg, 100)

	go bcast.Transmitter(17101, sendTX, sendOrderTX)
	go bcast.Receiver(17101, receiveTX, receiveOrderTX)

	elev_copy, _ := elevator.DeepCopyElevator(elev)

	go distributor.Distributor(peerUpdateCh, sendTX, receiveTX, elev_copy, elevChan, buttonEventChan, orderChan, completedChan, sendOrderTX, receiveOrderTX, petWatchDogChan, watchDogStuckChan)

	select {}
}
