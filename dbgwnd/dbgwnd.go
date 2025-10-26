package dbgwnd

import (
	"faf-pioneer/applog"
	"faf-pioneer/adapter"
	"strings"
	"github.com/pion/webrtc/v4"
	"strconv"
	"github.com/goforj/godump"
	"os"
	"time"
	"encoding/json"
	"slices"
	. "modernc.org/tk9.0"
	_ "modernc.org/tk9.0/themes/azure"
)

var logStrings 			[]string
var formattedLogs		[]string
var rawLogs             []string
var sortedIds           []string
var allPeersStats       map[string]webrtc.StatsReport
var connectionStates    map[string]string

var logFilePath = ""
var formattedLogsCount = 0
var rawLogCount = 0
var logViewLinesCount = 0
var currentNumOfIds = 0
var treeViewNumOfLines = 0
var showRawLogs = false
var hideDebug = false
var clearLogWindow = false
var connListBoxFirstUpdateDone = false

var myApp  = App
var connTreeView        *TTreeviewWidget
var connTreeScroll      *TScrollbarWidget
var notebook            *TNotebookWidget
var logFrame            *TFrameWidget
var connInfoFrame       *TFrameWidget
var logView 			*TextWidget
var logScroll 			*TScrollbarWidget
var logXScroll 			*TScrollbarWidget
var rawLogsChkButton 	*CheckbuttonWidget
var hideDebugChkButton  *CheckbuttonWidget
var connInfoView 		*TextWidget
var connInfoScroll 		*TScrollbarWidget
var connIdListBox       *ListboxWidget

var rawLogsChButtonVar  bool
var hideDebugButtonVar  bool


var FieldsAsJsonLog = false

func CreateMainWindow(){
	myApp.WmTitle("FAF Pioneer")
	
	// Connections treeview
	connTreeView = TTreeview(Selectmode("browse"), Columns("1 2 3 4"), Height(10), 
		Yscrollcommand(func(e *Event) { e.ScrollSet(connTreeScroll) }))

	connTreeView.Column("#0", Anchor("center"), Width(60))
	connTreeView.Column(1, Anchor("center"), Width(60))
	connTreeView.Column(2, Anchor("center"), Width(60))
	connTreeView.Column(3, Anchor("center"), Width(60))
	connTreeView.Column(4, Anchor("center"), Width(60))

	connTreeView.Heading("#0", Txt("      ID"), Anchor("center"))
	connTreeView.Heading(1, Txt("Connection state"), Anchor("center"))
	connTreeView.Heading(2, Txt("RTT(ping)"), Anchor("center"))
	connTreeView.Heading(3, Txt("Local"), Anchor("center"))
	connTreeView.Heading(4, Txt("Remote"), Anchor("center"))

	connTreeScroll = TScrollbar(Command(func(e *Event) { e.Yview(connTreeView) }))

	
	// notebook contains different tabs (Logs, conn info etc.)
	notebook = TNotebook()
	logFrame = TFrame()
	connInfoFrame = TFrame()

	notebook.Add(logFrame, Txt("Logs"))
	notebook.Add(connInfoFrame, Txt("Connection info"))
	GridRowConfigure(logFrame, 0, Weight(1))
	GridColumnConfigure(logFrame, 0, Weight(1))
	GridRowConfigure(connInfoFrame, 0, Weight(1))
	GridColumnConfigure(connInfoFrame, 0, Weight(1))
	
	//Logs tab 
	logView = logFrame.Text(Wrap("none"), Setgrid(true), Yscrollcommand(
		func(e *Event) { e.ScrollSet(logScroll) }))
	logScroll = logFrame.TScrollbar(Command(func(e *Event) { e.Yview(logView) }))
	logXScroll = logFrame.TScrollbar(Command(func(e *Event) { e.Xview(logView) }), Orient("horizontal"))

	rawLogsChkButton = logFrame.Checkbutton(Txt("Show unformatted logs"), Variable(rawLogsChButtonVar), 
	Onvalue(true), Offvalue(false), Command(func() { rawLogsChkButtonPressed() }))

	hideDebugChkButton = logFrame.Checkbutton(Txt("Hide DEBUG"), Variable(hideDebugButtonVar), 
	Onvalue(true), Offvalue(false), Command(func() { hideDebugChkButtonPressed() }))

	Grid(logView, Row(0), Sticky("NSWE"))
	Grid(logScroll, Row(0), Column(1), Sticky("NSWE"), Pady("2m"))
	Grid(logXScroll, Row(1), Column(0), Sticky("NSWE"), Pady("2m"))
	Grid(rawLogsChkButton, Row(2), Column(0), Sticky("W"), Pady("1m"))
	Grid(hideDebugChkButton, Row(2), Column(0), Sticky("W"), Pady("1m"), Padx("50m"))

	//Connection info tab
	connInfoView = connInfoFrame.Text(Wrap("none"), Setgrid(true), Yscrollcommand(
		func(e *Event) { e.ScrollSet(connInfoScroll) }))
	connInfoScroll = connInfoFrame.TScrollbar(Command(func(e *Event) { e.Yview(connInfoView) }))
	connIdListBox = connInfoFrame.Listbox()
	Bind(connIdListBox, "<<ListboxSelect>>", Command(infoListIdSelected))

 
	Grid(connIdListBox, Row(0), Column(0), Sticky("NSW"))
	Grid(connInfoView, Row(0), Column(0), Columnspan(2), Sticky("NSWE"), Padx("40m"))
	Grid(connInfoScroll, Row(0), Column(2), Sticky("NSWE"), Pady("2m"))

	Grid(connTreeView, Row(0), Sticky("NSWE"), Pady("2m"), Ipadx("1m"), Ipady("1m"))
	Grid(connTreeScroll, Row(0), Column(1), Sticky("NES"), Pady("2m"))
	Grid(notebook, Row(1), Sticky("NSWE"))
	GridRowConfigure(myApp, 0, Weight(1))
	GridColumnConfigure(myApp, 0, Weight(1))

	ActivateTheme("azure light")

	refreshUI()

	Bind(myApp, "<Escape>", Command(func() { refreshConnStats() }))

	myApp.Wait()
}

func rawLogsChkButtonPressed(){
	clearLogWindow = true
	if rawLogsChkButton.Variable() == "true" {
		showRawLogs = true
	} else {
		showRawLogs = false
	}
}

func hideDebugChkButtonPressed(){
	clearLogWindow = true
	if hideDebugChkButton.Variable() == "true" {
		hideDebug = true
	} else {
		hideDebug = false
	}
}

func infoListIdSelected(){
	selected := connIdListBox.Curselection()

	if len(selected) > 0 {
		selectedId := connIdListBox.Get( strconv.Itoa(selected[0]))
		if len(selectedId) > 0 {
			stat, ok := allPeersStats[selectedId[0]]

			if ok {
				connInfoView.Clear()
				title := "Connection ID: " + selectedId[0] + " Time: " + time.Now().Format("15:04:05")

				connInfoView.Insert(END,  title, "", "\n")
				connInfoView.Insert(END,  godump.DumpJSONStr(stat) +" ", "", "\n")
			}
		}
	}
}

func refreshUI() {
	refreshLogs()
	refreshConnStats()
	TclAfter(time.Second * 1, refreshUI)
}

func refreshConnInfoListIds() {
	if currentNumOfIds != 0 {
		connIdListBox.Delete("0", strconv.Itoa(currentNumOfIds - 1))
	}
	currentNumOfIds = len(sortedIds)

	for i := len(sortedIds) - 1; i >= 0; i-- {
		connIdListBox.Insert(0, sortedIds[i])
	}
}

func refreshLogs() {
	textYpos,_ := strconv.ParseFloat(strings.Split(logView.Yview(), " ")[1], 64)

	if logFilePath == "" {
		logFilePath = applog.GetLogFilePath()
	}
	contentBytes, err := os.ReadFile(logFilePath)

	if err == nil {
		logStrings = nil
		fileContent := string(contentBytes)
		logStrings = strings.Split(fileContent,"\n")
		formatLogLines()
	}
	
	if clearLogWindow {
		logView.Clear()
		logViewLinesCount = 0
		clearLogWindow = false
	}

	if showRawLogs {
		for i, st := range rawLogs {
			if i < logViewLinesCount {
				continue
			}

			logViewLinesCount += 1
			logView.Insert(END, st+" ", "", "\n")
		}
	} else {
		for i, st := range formattedLogs {
			if i < logViewLinesCount {
				continue
			}

			logViewLinesCount += 1

			if hideDebug {
				if !strings.Contains(st, "DEBUG"){
					logView.Insert(END, st+" ", "", "\n")
				}
			} else {
				logView.Insert(END, st+" ", "", "\n")
			}
		}
	}
	

	//autoscroll when close to bottom and also on app start
	if textYpos > 0.99 || logViewLinesCount < 30{
		logView.Yviewmoveto(1)
	}
}

func formatLogLines() {
	if showRawLogs {
		for i, st := range logStrings {
			if i < rawLogCount {
				continue
			}

			// there should be no empty lines and if there is one
			// then something went wrong and we skip this try
			if len(st) < 5 {
				break
			}
			rawLogCount += 1
			rawLogs = append(rawLogs, st)
		}

	} else {
		for i, st := range logStrings {
			if i < formattedLogsCount {
				continue
			}

			if len(st) < 5 {
				break
			}

			formattedLogsCount += 1
			formattedLogs = append(formattedLogs, st)

			//replace loglevel with capital string
			formattedLogs[i] = strings.ReplaceAll(st,`"level":"info"`, "INFO")
			formattedLogs[i] = strings.ReplaceAll(formattedLogs[i],`"level":"debug"`, "DEBUG")
			formattedLogs[i] = strings.ReplaceAll(formattedLogs[i],`"level":"error"`, "ERROR")
			formattedLogs[i] = strings.ReplaceAll(formattedLogs[i],`"level":"warn"`, "WARN")

			//Replace date+time with local hh-mm-ss
			index := strings.Index(formattedLogs[i], `Z"`)
			if index != -1 {
				localTime := time.Now().Format("15:04:05")

				st1 := formattedLogs[i][:index-27] + localTime + formattedLogs[i][index+1:]
				formattedLogs[i] = st1
			}

			//Remove caller
			index = strings.Index(formattedLogs[i], `"caller"`)
			if index != -1 {
				ind2 := strings.Index(formattedLogs[i], `"msg"`)
				if ind2 != -1 {
					st1 := formattedLogs[i][:index] + formattedLogs[i][ind2+5:]
					formattedLogs[i] = st1
				}
			}

			//remove userID & localGameId
			if !strings.Contains(formattedLogs[i], `"Application started"`) {
				index = strings.Index(formattedLogs[i], `"localUserId"`)
				if index != -1{
					ind2 := strings.Index(formattedLogs[i], `"localGameId"`)
					if ind2 != -1 {
						indEnd1 := strings.Index(formattedLogs[i][ind2:], `,`)
						indEnd2 := strings.Index(formattedLogs[i][ind2:], `}`)
						indEndFinal := 0
						if indEnd1 == -1{
							indEndFinal = indEnd2
						} else if indEnd2 == -1 {
							indEndFinal = indEnd1
						} else {
							indEndFinal = min(indEnd1, indEnd2)
						}
					
						st1 := formattedLogs[i][:index] + formattedLogs[i][indEndFinal+ind2:]
						formattedLogs[i] = st1
					}
				}
			}
		}
	}	
}

func refreshConnStats(){
	pm := adapter.GetPeerManager()
	if pm != nil {
		allPeersStats, connectionStates = pm.GetAllPeersStats()

		//sort all ids so the they always displayed in the right order
		sortedIds = nil
		sortedIdsInt := make([]int, 1)
		for id, _ := range(allPeersStats) {
			i,err := strconv.Atoi(id)
			if err == nil {
				sortedIdsInt = append(sortedIdsInt, i)
			}
		}
		slices.Sort(sortedIdsInt)
		for _,id := range(sortedIdsInt) {
			if id != 0{
				sortedIds = append(sortedIds, strconv.Itoa(id))
			}
		}

		if !connListBoxFirstUpdateDone{
			if len(allPeersStats) != currentNumOfIds {
				refreshConnInfoListIds()
			}
		}
	}

	if len(allPeersStats) > 0 {
		// delete all lines in treeView. Idk if they are updatable or not
		// but for now we just delete all and add them again with new data
		for treeViewNumOfLines > 0 {
			connTreeView.Delete(strconv.Itoa(treeViewNumOfLines))
			treeViewNumOfLines -= 1
		}		
		
		for _, id := range(sortedIds) {
			if stats, ok := allPeersStats[id]; ok {
				localCandidates := make(map[string]string)
				remoteCandidates := make(map[string]string)
				connectionState := "-"
				activeLocalCandidateId := ""
				activeRemoteCandidateId := ""
				activeLocalCandiType := "-"
				activeRemoteCandiType := "-"
				ping := "-"

				if cstate, ok := connectionStates[id]; ok {
					connectionState = cstate
				}
				
				for k,_ := range(stats) {
					//I'll take this data no matter what!
					//ARE YOU HEAR ME, GO?
					dataAsJSON := godump.DumpJSONStr(stats[k])
					var result map[string]interface{}
					err := json.Unmarshal([]byte(dataAsJSON), &result)

					if err == nil {
						if typ, ok := result["type"].(string); ok {
							if idVal,ok := result["id"].(string); ok {
								switch typ {
									case "local-candidate":
										if candidateType,ok := result["candidateType"].(string); ok {
											localCandidates[idVal] = candidateType
										}
									case "remote-candidate":
										if candidateType,ok := result["candidateType"].(string); ok {
											remoteCandidates[idVal] = candidateType
										}
									case "candidate-pair":
										if nominated,ok := result["nominated"].(bool); ok {
											if nominated {
												if localCandi,ok := result["localCandidateId"].(string); ok {
													activeLocalCandidateId = localCandi
												}
												if remoteCandi,ok := result["remoteCandidateId"].(string); ok {
													activeRemoteCandidateId = remoteCandi
												}
											
												if rtt, ok := result["currentRoundTripTime"].(float64); ok {
													ping = strconv.Itoa(int(rtt * 1000))
												}
											}
										}	
								}
							}
						}
					}
					
				}
				
				if localCandiType, ok := localCandidates[activeLocalCandidateId]; ok {
					activeLocalCandiType = localCandiType
				}
				if remoteCandiType, ok := remoteCandidates[activeRemoteCandidateId]; ok {
					activeRemoteCandiType = remoteCandiType
				}

				treeViewNumOfLines += 1

				//https://gitlab.com/cznic/tk9.0/-/blob/master/themes/azure/_examples/example.go#L205
				tvData := []any{"", treeViewNumOfLines, id,
					"{"+connectionState+"}"+" "+"{"+ping+"}"+" "+"{"+activeLocalCandiType+"}"+" "+"{"+activeRemoteCandiType+"}"}

				connTreeView.Insert(tvData[0], "end", Id(tvData[1]), Txt(tvData[2]), Value(tvData[3]))
			}
		}
	}
}