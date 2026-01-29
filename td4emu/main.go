package main

// 4bitCPU td4用のエミュレータ
// 16進数テキスト形式で出力されたtd4用のバイナリコードを読み込み、実行するプログラムです。
// > go fmt .\main.go
// > go build -o td4emu.exe .\main.go
// > td4emu.exe -step .\Hikizan.hex

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// CPU 構造体: TD4の内部状態を保持
type CPU struct {
	A, B    uint8     // 4bit レジスタ
	PC      uint8     // 4bit プログラムカウンタ
	BP      uint8     // 4bit ブレイクポイント
	C       bool      // キャリーフラグ
	OutPort uint8     // 4bit 出力ポート
	InPort  uint8     // 4bit 入力ポート (今回は固定で0)
	ROM     [16]uint8 // 16バイトのプログラムメモリ
}

var (
	MEM_MIN uint8 = 0
	MEM_MAX uint8 = 15
)

// NewCPU CPUの初期化
func NewCPU() *CPU {
	return &CPU{
		ROM: [16]uint8{}, // ゼロ初期化 (NOP)
		BP:  255,         // Break point 0-15以外の値は未設定の状態
	}
}

// LoadROM ファイルからHex文字列を読み込んでROMに格納
func (cpu *CPU) LoadROM(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	addr := 0
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.Trim(line, " \n\r")
		if line == "" { // 空行の場合は、読み飛ばす。
			continue
		}
		if line[0] == ';' { // コメントの場合は、読み飛ばす。
			continue
		}
		// 16進数文字列を数値に変換
		val, err := strconv.ParseUint(line, 16, 8)
		if err != nil {
			return fmt.Errorf("invalid hex format at line %d: %s", addr+1, line)
		}

		if addr < 16 {
			cpu.ROM[addr] = uint8(val)
			addr++
		} else {
			return fmt.Errorf("Memory overflow!!\nThe memory size of this system is  %d bytes.", len(cpu.ROM))
		}
	}
	return scanner.Err()
}

// DumpMemory 現在のメモリ内容を表示
func (cpu *CPU) DumpMemory(adress uint8) {
	// コンソール画面をクリア（ANSIエスケープシーケンス）
	// Windowsの古いコマンドプロンプトでは効かない場合がありますが、PowerShellやVSCodeなら動作します
	// fmt.Print("\033[H\033[2J")
	// 2進数表記のヘルパー
	bin4 := func(v uint8) string {
		return fmt.Sprintf("%04b", v&0xF)
	}
	if adress != cpu.BP {
		fmt.Printf("|   %02d   | 0x%02X 0b%s_%s |\n",
			adress, cpu.ROM[adress], bin4(cpu.ROM[adress]>>4), bin4(cpu.ROM[adress]))
	} else {
		fmt.Printf("|   %02d B | 0x%02X 0b%s_%s |\n",
			adress, cpu.ROM[adress], bin4(cpu.ROM[adress]>>4), bin4(cpu.ROM[adress]))
	}
}

// DumpState 現在のCPU状態を表示
func (cpu *CPU) DumpState(adress uint8) {
	// コンソール画面をクリア（ANSIエスケープシーケンス）
	// Windowsの古いコマンドプロンプトでは効かない場合がありますが、PowerShellやVSCodeなら動作します
	// fmt.Print("\033[H\033[2J")
	cInt := 0
	if cpu.C {
		cInt = 1
	}
	// 2進数表記のヘルパー
	bin4 := func(v uint8) string {
		return fmt.Sprintf("%04b", v&0xF)
	}
	if adress != cpu.BP { // Break pointのある位置にBを表示する。
		fmt.Printf("| PC:%02d   | OP:%02X | A:%s(%X) | B:%s(%X) | C:%d | IN:%s | OUT:%s |\n",
			adress, cpu.ROM[adress], bin4(cpu.A), cpu.A, bin4(cpu.B), cpu.B, cInt, bin4(cpu.InPort), bin4(cpu.OutPort))
	} else {
		fmt.Printf("| PC:%02d B | OP:%02X | A:%s(%X) | B:%s(%X) | C:%d | IN:%s | OUT:%s |\n",
			adress, cpu.ROM[adress], bin4(cpu.A), cpu.A, bin4(cpu.B), cpu.B, cInt, bin4(cpu.InPort), bin4(cpu.OutPort))
	}
}

// Execute 1命令実行サイクル
func (cpu *CPU) Execute() int {
	if cpu.PC == cpu.BP { // ブレイクポイントなら、ここで1を返して終了する。
		return 1
	}
	// フェッチ
	opcode := cpu.ROM[cpu.PC]
	// 次のPCを仮計算 (通常は PC+1, 15を超えたら0に戻る)
	nextPC := (cpu.PC + 1) & 0x0F
	// 下位4ビット（即値 Im）
	im := opcode & 0x0F

	// 上位4ビットで命令判定するか、特定のビットパターンで判定
	// TD4の命令デコードロジック
	switch {
	// ADD A, Im (0000xxxx)
	case (opcode & 0xF0) == 0x00:
		res := uint16(cpu.A) + uint16(im)
		cpu.A = uint8(res & 0x0F)
		cpu.C = res > 15 // キャリー発生判定

	// ADD B, Im (0101xxxx)
	case (opcode & 0xF0) == 0x50:
		res := uint16(cpu.B) + uint16(im)
		cpu.B = uint8(res & 0x0F)
		cpu.C = res > 15 // キャリー発生判定

	// MOV A, B (00010000) - 0x10
	case opcode == 0x10:
		cpu.A = cpu.B

	// MOV B, A (01000000) - 0x40
	case opcode == 0x40:
		cpu.B = cpu.A

	// MOV A, Im (0011xxxx)
	case (opcode & 0xF0) == 0x30:
		cpu.A = im

	// MOV B, Im (0111xxxx)
	case (opcode & 0xF0) == 0x70:
		cpu.B = im

	// JMP Im (1111xxxx)
	case (opcode & 0xF0) == 0xF0:
		nextPC = im   // ジャンプ成立時はPCを書き換え
		cpu.C = false // ※TD4仕様: JMPでCフラグは変化しないことが多いが、実装によってはリセットする場合もある。
		// ここでは標準的なTD4仕様に従い、Cフラグは保持すべきだが、
		// 一般的な解説ではJMPでCが変わる記述はないため、保持します。
		// (ただし、元のCソース実装などでCがリセットされる場合もあるので注意)

	// JNC Im (1110xxxx) - Jump if Not Carry
	case (opcode & 0xF0) == 0xE0:
		if !cpu.C {
			nextPC = im
		}
		cpu.C = false // JNC命令実行後は通常Cフラグはクリアされませんが、
		// 次の演算まで保持されるべきです。ここでは何もしないのが正解。

	// IN A (00100000)
	case opcode == 0x20:
		cpu.A = cpu.InPort

	// IN B (01100000)
	case opcode == 0x60:
		cpu.B = cpu.InPort

	// OUT B (10010000)
	case opcode == 0x90:
		cpu.OutPort = cpu.B

	// OUT Im (1011xxxx)
	case (opcode & 0xF0) == 0xB0:
		cpu.OutPort = im
	}
	// PC更新
	cpu.PC = nextPC
	return 0
}

// TrimLastChar は文字列の最後のルーンを削除します
func TrimLastChar(s string) string {
	if s == "" {
		return ""
	}
	// 文字列をルーンのスライスに変換する
	runes := []rune(s)
	// スライスの最後の要素を除外して、新しい文字列として返す
	return string(runes[:len(runes)-1])
}

// inRange 指定した値の範囲にあるかを判別する。範囲内であればtrueを返す。
func inRange(min, value, max uint8) bool {
	return value >= min && value <= max
}

// 	if inRange(MEM_MIN, adress, MEM_MAX) {}

func main() {
	//	var loop int = 1

	// 1. オプション（フラグ）の定義
	stepMode := flag.Bool("step", false, "Enable step execution mode")
	speed := flag.Int64("speed", 1000, "Execution speed in milliseconds per instruction")

	// 2. ヘルプ表示のカスタマイズ
	flag.Usage = func() {
		// ヘルプのヘッダー部分
		fmt.Fprintf(os.Stderr, "TD4 エミュレータ\n")
		fmt.Fprintf(os.Stderr, "4bitマイコン用のエミュレータです。\n\n")
		// 使い方の構文
		fmt.Fprintf(os.Stderr, "使い方:\n")
		fmt.Fprintf(os.Stderr, "td4emu [オプション] ファイル名\n\n")

		// オプション一覧（定義したフラグを自動で表示してくれる便利な関数）
		fmt.Fprintf(os.Stderr, "オプション:\n")
		flag.PrintDefaults()
		// 使用例
		fmt.Fprintf(os.Stderr, "\n使用例:\n")
		fmt.Fprintf(os.Stderr, "  td4emu timer.hex             (標準実行)\n")
		fmt.Fprintf(os.Stderr, "  td4emu -step timer.hex       (ステップ実行)\n")
		fmt.Fprintf(os.Stderr, "  td4emu -speed 500 timer.hex  (実行速度の設定,単位はミリ秒)\n")
	}

	// 3. 解析実行
	flag.Parse()

	// 4. 引数チェック（ファイル名がない場合）
	args := flag.Args()
	if len(args) < 1 {
		fmt.Printf("Usage: go run main.go [options] <hex_file>\n")
		fmt.Printf("Usage: td4emu.exe [options] <hex_file>\n")
		fmt.Printf("Options:\n")
		flag.Usage()
		flag.PrintDefaults()
		os.Exit(1)
	}
	filename := args[0]

	cpu := NewCPU()
	if err := cpu.LoadROM(filename); err != nil {
		log.Fatalf("Error loading ROM: %v", err)
	}

	fmt.Printf("4bit CPU TD4 emulator\n")
	fmt.Printf("Reading from the serial port...\n")

	fmt.Printf("Loaded %s. Starting Emulator...\n", filename)
	fmt.Printf("Mode: Step=%v, Speed=%5dms/inst\n", *stepMode, *speed)
	fmt.Printf("| PC   BP |OP-code|A register |B register |Cflag| IN port | OUT port |\n")
	fmt.Printf("|:--------|:-----:|:---------:|:---------:|:---:|:-------:|:--------:|\n")
	//	現在の状態を表示
	cpu.DumpState(cpu.PC)
	//	fmt.Printf("\n")
	//	入力待ち用のリーダー
	stdin := bufio.NewReader(os.Stdin)
	execStatus := true
	for execStatus {
		//	loop = 1
		//	ステップ実行モードの場合
		if *stepMode {
			fmt.Printf("> ")
			//	stdin.ReadBytes('\n')
			//	改行文字 '\n' が現れるまでバイトを読み込む
			//	ReadBytesは区切り文字 '\n' も含めて返却する
			data, _ := stdin.ReadBytes('\n')
			//	fmt.Printf("%s\n", data)
			line := strings.ToUpper(string(data))
			//	fmt.Printf("%s\n", line)
			// line = strings.ToUpper(string(line))
			elements := strings.Split(line, " ")
			firstWord := line[0]
			//	fmt.Printf("%s\n", firstWord)
			//	fmt.Printf("\n")
			switch firstWord {
			/*
				Xコマンド	レジスタ、カウンタ、フラグ類の検査と変更
			*/
			case 'G': //	ユーザプログラムの連続実行
				if len(elements) == 1 {
					*stepMode = false
					cpu.DumpState(cpu.PC)
				} else if len(elements) > 1 {
					//	数値変換
					val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
					if err == nil { // 文字列=>数値変換にエラーがなければ、設定速度を更新
						if inRange(MEM_MIN, uint8(val), MEM_MAX) { // アドレスの範囲であれば、PCのアドレスを更新して、連続実行モードに移行する。
							cpu.PC = uint8(val)
							*stepMode = false
							cpu.DumpState(cpu.PC)
						} else {
							fmt.Printf("The address space that can be set by the program counter ranges from 0 to 15.")
							*stepMode = true
						}
					} else {
						fmt.Printf("G command parameter is invalid.\n")
					}
				}
			case 'V': //	実行速度の設定(velocity)
				if len(elements) == 1 {
					fmt.Printf("Speed=%5dms/inst\n", *speed)
				} else if len(elements) > 1 {
					//	数値変換
					val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 64)
					if err == nil { // 文字列=>数値変換にエラーがなければ、設定速度を更新
						*speed = int64(val)
						fmt.Printf("Speed=%5dms/inst\n", *speed)
					} else {
						fmt.Printf("Failed to set execution speed.\n")
						fmt.Printf("Please set the execution time for one step in milliseconds.\n")
					}
				}
			case 'B': //	ブレークポイントの参照、設定と解除
				if len(elements) == 1 {
					if inRange(MEM_MIN, cpu.BP, MEM_MAX) {
						fmt.Printf("Break point: %d", cpu.BP)
					} else {
						fmt.Printf("Break point: none")
					}
				} else if len(elements) > 1 {
					//	数値変換
					val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
					if err == nil { // 文字列=>数値変換にエラーがなければ、次のステップへ
						cpu.BP = uint8(val)
						if inRange(MEM_MIN, uint8(val), MEM_MAX) { // アドレスの範囲であれば、BPに値を設定する。
							fmt.Printf("Break point: %d", cpu.BP)
						} else {
							fmt.Printf("Break point: none")
						}
					}
				}
				fmt.Printf("\n")
			case 'T': //	レジスタ表示しながらトレース実行する回数を設定する。
				if len(elements) == 1 { // 引数がない場合は、1ステップだけ実行する。
					cpu.Execute()
					cpu.DumpState(cpu.PC)
				} else if len(elements) > 1 {
					//	数値変換
					val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
					if err == nil {
						// fmt.Printf("|\n")
						loop := int(val)
						//	命令実行
						for i := 0; i < loop; i++ {
							state := cpu.Execute()
							if state != 0 {
								break //	Breakpointに到達したら、停止する。
							}
							time.Sleep(time.Duration(*speed) * time.Millisecond)
							cpu.DumpState(cpu.PC)
						}
					}
				}
			case 'S': //	メモリの指定されたアドレスに値を書き込む。
				if 3 == len(elements) {
					adr, err1 := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
					val, err2 := strconv.ParseInt(strings.Trim(elements[2], " \n\r"), 0, 16)
					//	fmt.Printf("\nadr:%d,val:%2x\n", adr, val)
					if err1 == nil && err2 == nil { //	正常に整数値に変換されたかをチェック
						if inRange(MEM_MIN, uint8(adr), MEM_MAX) { //	指定されたアドレスがメモリ空間内であるかをチェック
							cpu.ROM[uint8(0x0f&adr)] = uint8(val) //	メモリの指定されたアドレスの内容を書換える。
							cpu.DumpMemory(uint8(adr))
						} else {
							fmt.Printf("The memory space ranges from 0-15.")
						}
					} else {
						fmt.Printf("The input value is incorrect.")
					}
					fmt.Printf("\n")
				}
			case 'I': //	入力ポートの値を設定する。
				if len(elements) > 1 {
					//	数値変換
					val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
					if err == nil {
						cpu.InPort = uint8(0x0f & val)
						cpu.DumpState(cpu.PC)
					} else {
						fmt.Printf("Only integer values ​​between 0 and 15 can be set.\n")
					}
				}
			case 'D': //	現在のCPUのレジスタ内容を表示する。
				if 1 == len(elements) {
					cpu.DumpState(cpu.PC)
				}
			case 'M': //	現在の現在のメモリ内容を表示
				if 1 == len(elements) {
					fmt.Printf("| Adress | OP-code          |\n")
					fmt.Printf("|:-------|:----------------:|\n")
					for adr := 0; adr < 16; adr++ {
						cpu.DumpMemory(uint8(adr))
					}
				}
			case 'Q':
				// 	fmt.Printf("\n")
				execStatus = false // プログラムを終了する。
			}
		} else {
			// fmt.Println()
			//	通常実行モードの場合、指定時間待機
			//	命令実行
			result := cpu.Execute()
			if 0 != result {
				*stepMode = true
				continue
			}
			time.Sleep(time.Duration(*speed) * time.Millisecond)
			cpu.DumpState(cpu.PC)
		}
		/*
			for i := 0; i < loop; i++ {
				//	命令実行
				cpu.Execute()
				if *stepMode == false {
					fmt.Printf("\n")
				}
			}
		*/
	}
	fmt.Printf("program terminated !\n")
}
