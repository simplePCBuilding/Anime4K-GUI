package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	g "github.com/AllenDang/giu"
	"github.com/AllenDang/imgui-go"
	"gopkg.in/vansante/go-ffprobe.v2"
)

func loop() {
	resolutionsNames := make([]string, len(resolutions))
	for index, res := range resolutions {
		resolutionsNames[index] = res.Format()
	}

	shadersNames := make([]string, len(shadersModes))
	for index, shaders := range shadersModes {
		shadersNames[index] = shaders.Name
	}

	compressionNames := make([]string, len(compressionPresets))
	for index, compression := range compressionPresets {
		compressionNames[index] = compression.Name
	}

	g.SingleWindow().Layout(
		g.SplitLayout(g.DirectionVertical, 515,
			g.SplitLayout(g.DirectionHorizontal, 1200,
				g.Layout{
					g.Table().Flags(g.TableFlagsResizable).Rows(buildTableRows()...).Columns(buildTableColumns()...),
				},
				g.Layout{
					g.Label("Settings"),
					g.Label(""),

					g.Label("Target resolution"),
					g.Combo("", resolutionsNames[selectedResolution], resolutionsNames, &selectedResolution).Size(400),
					g.Label(""),

					g.Label("Shaders mode"),
					g.Tooltip("Check the project's GitHub page if you're not sure what to choose"),
					g.Combo("", shadersModes[selectedShadersMode].Name, shadersNames, &selectedShadersMode).Size(400),
					g.Tooltip("Check the project's GitHub page if you're not sure what to choose"),
					g.Label(""),

					g.Label("Compression level"),
					g.Combo("", compressionNames[selectedCompressionPreset], compressionNames, &selectedCompressionPreset).Size(400),
					g.Label(""),

					g.Label("Output format"),
					g.Combo("", outputFormats[selectedOutputFormat], outputFormats, &selectedOutputFormat).Size(400),
					g.Label(""),

					g.Checkbox("Disable hardware acceleration", &disableHardwareAcceleration),
					g.Tooltip("Should be used only for bad performance or compatibility issues"),

					g.Checkbox("Disable hardware accelerated encoding (NVENC / OpenCL)", &disableHardwareAccelerationEncoding),
					g.Tooltip("Can be used to encode with H.264"),

					g.Checkbox("Debug mode", &debug),
					g.Tooltip("Show more detailed logs, useful for troubleshooting and debugging"),

					g.Label(""),

					g.Button(buttonLabel).OnClick(handleButton).Size(360, 30),

					g.Label(""),
					g.Label("GPU Usage: " + gpuUsage),
					g.Label("VRAM Usage: " + vramUsage),
					g.Custom(func() {
						if hwaccelValue != "cuda" {
							return
						}

						g.Label("GPU Temperature: " + gpuTemperature).Build()
					}),
				},
			),
			g.Layout{
				g.Label("Logs"),
				g.InputTextMultiline(&logs).Flags(g.InputTextFlagsReadOnly).Size(1600, 337),
				g.SplitLayout(g.DirectionHorizontal, 1330,
					g.SplitLayout(g.DirectionHorizontal, 80,
						g.Label("Progress: "),
						g.ProgressBar(progress).Overlay(progressLabel).Size(1230, 20),
					),
					g.SplitLayout(g.DirectionHorizontal, 110,
						g.Label(currentSpeed),
						g.Label(currentTime),
					),
				),
			},
		),
	)
}

func handleDrop(files []string) {
	if processing {
		return
	}

	ffprobe.SetFFProbeBinPath(".\\ffmpeg\\ffprobe.exe")
	ctx, closeCtx := context.WithTimeout(context.Background(), 5*time.Second)
	defer closeCtx()

LOOP:
	for _, path := range files {
		if !strings.HasSuffix(path, ".mp4") && !strings.HasSuffix(path, ".avi") && !strings.HasSuffix(path, ".mkv") {
			logMessage("Invalid input file format (supported: mp4, avi, mkv)! Path: "+path, false)
			continue
		}

		for _, anime := range animeList {
			if anime.Path == path {
				logMessage("File is already added to queue, ignoring it", false)
				continue LOOP
			}
		}

		file, err := os.Stat(path)
		if err != nil {
			handleSoftError("Reading file stats error", err.Error())
			return
		}

		data, err := ffprobe.ProbeURL(ctx, path)
		if err != nil {
			handleSoftError("FFPROBE error", err.Error())
			return
		}

		split := strings.Split(path, string(os.PathSeparator))
		anime := Anime{
			Name:   split[len(split)-1],
			Length: int64(data.Format.DurationSeconds * 1000),
			Size:   file.Size(),
			Width:  data.FirstVideoStream().Width,
			Height: data.FirstVideoStream().Height,
			Path:   path,
			Status: NotStarted,
		}

		animeList = append(animeList, anime)
		progressLabel = fmt.Sprintf("%d / %d", calcFinished(), len(animeList))
		logMessage("Added file "+path, false)
	}
}

func handleButton() {
	if processing {
		cancelProcessing()
	} else {
		go startProcessing()
	}
}

func updateUI() {
	currentSpeed = "Speed:"
	currentTime = "Time:"
	progress = float32(calcFinished()) / float32(len(animeList))
	progressLabel = fmt.Sprintf("%d / %d", calcFinished(), len(animeList))
	g.Update()
}

func removeAnime(index int) {
	anime := animeList[index]
	animeList = append(animeList[:index], animeList[index+1:]...)
	updateUI()
	logMessage(fmt.Sprintf("Removed %s from queue", anime.Name), false)
}

func buildTableRows() []*g.TableRowWidget {
	rows := make([]*g.TableRowWidget, len(animeList))

	for i, anime := range animeList {
		rows[i] = g.TableRow(
			g.Label(fmt.Sprintf("%d", i+1)),
			g.Label(anime.Name),
			g.Label(formatMillis(anime.Length)),
			g.Label(formatMegabytes(anime.Size)),
			g.Label(fmt.Sprintf("%dx%d", anime.Width, anime.Height)),
			g.Label(string(anime.Status)),
			g.Custom(func() { // Workaround for weird UI bug
				g.Button("Remove").Disabled(processing).OnClick(func() { removeAnime(i) }).Build()
			}),
		)
	}

	return rows
}

func buildTableColumns() []*g.TableColumnWidget {
	columns := []*g.TableColumnWidget{
		g.TableColumn("ID").Flags(imgui.TableColumnFlags_WidthFixed).InnerWidthOrWeight(100),
		g.TableColumn("Title").Flags(imgui.TableColumnFlags_WidthFixed).InnerWidthOrWeight(550),
		g.TableColumn("Length").Flags(imgui.TableColumnFlags_WidthFixed).InnerWidthOrWeight(100),
		g.TableColumn("Size").Flags(imgui.TableColumnFlags_WidthFixed).InnerWidthOrWeight(100),
		g.TableColumn("Resolution").Flags(imgui.TableColumnFlags_WidthFixed).InnerWidthOrWeight(100),
		g.TableColumn("Status").Flags(imgui.TableColumnFlags_WidthFixed).InnerWidthOrWeight(100),
		g.TableColumn("Action").Flags(imgui.TableColumnFlags_WidthFixed).InnerWidthOrWeight(100),
	}

	return columns
}
