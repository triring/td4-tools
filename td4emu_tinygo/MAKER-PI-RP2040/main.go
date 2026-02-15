package main

// go fmt .\main.go
// tinygo flash -target=pico -size=short -monitor .
// tinygo build -o td4emu_tinygo.uf2 -target=pico -size=short .
// 4bitCPU td4用のエミュレータ

import (
	"bufio"
	"fmt"
	"machine"
	"os"
	"strconv"
	"strings"
	"time"
	/*
		"flag"
		"log"
		"time"
	*/)

// CPU 構造体: TD4の内部状態を保持
type CPU struct {
	A, B    uint8          // 4bit レジスタ
	PC      uint8          // 4bit プログラムカウンタ
	BP      uint8          // 4bit ブレイクポイント
	C       bool           // キャリーフラグ
	OutPort uint8          // 4bit 出力ポート
	InPort  uint8          // 4bit 入力ポート (今回は固定で0)
	ROM     [16]uint8      // 16バイトのプログラムメモリ
	led     [4]machine.Pin // ハードウェア上に接続されているledのPin情報
	sw      [2]machine.Pin // ハードウェア上に接続されているswのPin情報
}

var (
	MEM_MIN uint8 = 0
	MEM_MAX uint8 = 15
)

// var led machine.Pin // ledが接続されているピン
var HelpText = [...]string{
	"Command list",
	"\tH :(Help) コマンドの使用方法を表示する。",
	"\tS [address] [pocode] [pocode] ... :(Setdata) 指定したメモリ番地にオペコードを書き込む。",
	"\tB [address] :(Breakpoint) ブレークポイントの設定と削除を行う。",
	"\tM :(Memory) 現在の現在のメモリの内容を表示する。",
	"\tD :(Dump) 現在のCPUのレジスタ内容を表示する。",
	"\tT [count] :(Trace) プログラムを指定回数だけ命令を実行する（ステップ実行）。",
	"\tG [address] :(Go) 指定したアドレスからプログラムを実行する。",
	"\tV [speed] :(Velocity) 実行速度を設定する。",
	"\tI [bit pattern] :(InPort) 入力ポートの値を設定する。",
	"\tQ :(Quit) モニタプログラムを終了する。",
}

// NewCPU CPUの初期化
func NewCPU() *CPU {
	var led [4]machine.Pin
	var sw [2]machine.Pin

	led[0] = machine.GP0
	led[1] = machine.GP1
	led[2] = machine.GP2
	led[3] = machine.GP3
	led[0].Configure(machine.PinConfig{Mode: machine.PinOutput})
	led[1].Configure(machine.PinConfig{Mode: machine.PinOutput})
	led[2].Configure(machine.PinConfig{Mode: machine.PinOutput})
	led[3].Configure(machine.PinConfig{Mode: machine.PinOutput})

	sw[0] = machine.GP20
	sw[1] = machine.GP21
	sw[0].Configure(machine.PinConfig{Mode: machine.PinInput})
	sw[1].Configure(machine.PinConfig{Mode: machine.PinInput})

	return &CPU{
		ROM: [16]uint8{}, // ゼロ初期化 (NOP)
		BP:  255,         // Break point 0-15以外の値は未設定の状態
		led: led,
		sw:  sw,
	}
}

// LoadROM ファイルからHex文字列を読み込んでROMに格納
// 書式 S コマンドと同じ
// S adr opc1 opc2 opc3 ...
// 行の先頭は、S
// 2つ目は、書き込み開始アドレス
// それ以降に、書き込むバイナリデータ
// それぞれのデータ間は、スペースで区切る。
// LoadROM ファイルからHex文字列を読み込んでROMに格納
func (cpu *CPU) LoadROM(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	//	adr := 0
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.ToUpper(line)
		line = strings.Replace(line, ",", " ", -1)
		line = strings.Trim(line, " \n\r")
		if line == "" { // 空行の場合は、読み飛ばす。
			continue
		}
		if line[0] == ';' { // コメントの場合は、読み飛ばす。
			continue
		}
		if line[0] != 'S' {
			continue
		}
		// 要素に分割
		elements := strings.Split(line, " ")
		cpu.writeMemory(elements)
		break
	}
	return scanner.Err()
}

func (cpu *CPU) writeMemory(elements []string) {
	if 3 > len(elements) { // パラメータが足りない場合は、警告して終了
		fmt.Printf("Insufficient address or opcode information required for writing.\n")
		return
	} //	書き込み開始アドレスのデコード
	adr, adr_err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 16)
	if adr_err != nil { //	正常に整数値に変換されたかをチェック
		fmt.Printf("The address is specified incorrectly.\n")
		return
	}
	if false == inRange(MEM_MIN, uint8(adr), MEM_MAX) { //	指定されたアドレスがメモリ空間内であるかをチェック
		fmt.Printf("This system's memory space is limited to 0 to 15 bytes.\n")
		return
	}
	index := 2
	for {
		val, val_err := strconv.ParseInt(strings.Trim(elements[index], " \n\r"), 0, 16)
		//	fmt.Printf("%d %T\n",val, val_err)
		if val_err == nil { //	正常に整数値に変換されたかをチェック
			//	fmt.Printf("%x %x %T\n", adr, val, val_err)
			cpu.ROM[uint8(0x0f&adr)] = uint8(val) //	メモリの指定されたアドレスの内容を書換える。
			adr++
		} else {
			fmt.Printf("invalid hex format at %d: %s\n", index, elements[index])
			break
		}
		index++
		if index >= len(elements) {
			break
		}
		if uint8(adr) > MEM_MAX {
			fmt.Printf("Memory overflow!!\nThis system has only %d bytes of memory space.\n", len(cpu.ROM))
			break
		}
	}
	return
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
		input := 0
		for i := 0; i < 2; i++ {
			input = input << 1
			if !cpu.sw[i].Get() {
				input = input + 1
			} else {
				input = input + 0
			}
		}
		cpu.InPort = uint8(input)
		cpu.A = cpu.InPort

	// S 0x00 0x70 0x60 0x90 0xF0 [InOut.td4]
	// IN B (01100000)
	case opcode == 0x60:
		input := 0
		for i := 0; i < 2; i++ {
			input = input << 1
			if !cpu.sw[i].Get() {
				input = input + 1
			} else {
				input = input + 0
			}
		}
		cpu.InPort = uint8(input)
		cpu.B = cpu.InPort

	// OUT B (10010000)
	case opcode == 0x90:
		cpu.OutPort = cpu.B
		bit := 0x01
		for i := 0; i < 4; i++ {
			state := int(cpu.OutPort) & (bit << i)
			if 0 != state {
				cpu.led[i].High() //	fmt.Printf("On\n")
			} else {
				cpu.led[i].Low() //	fmt.Printf("Off\n")
			}
		}

	// OUT Im (1011xxxx)
	case (opcode & 0xF0) == 0xB0:
		cpu.OutPort = im
		bit := 0x01
		for i := 0; i < 4; i++ {
			state := int(cpu.OutPort) & (bit << i)
			if 0 != state {
				cpu.led[i].High() //	fmt.Printf("On\n")
			} else {
				cpu.led[i].Low() //	fmt.Printf("Off\n")
			}
		}
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

func main() {
	stepMode := true
	speed := 1000
	enter_flag := false // 改行コードのチェック用フラグ
	var readbuffer string
	time.Sleep(time.Millisecond * 2000)
	/*
		led = machine.LED
		led.Configure(machine.PinConfig{
			Mode: machine.PinOutput,
		})
	*/
	cpu := NewCPU() // TD4のオブジェクト生成および初期化
	// 接続されているLEDの点灯テスト
	for i := 0; i < 4; i++ {
		fmt.Printf("%T, %v\n", cpu.led[i], cpu.led[i])
		cpu.led[i].High() //	fmt.Printf("On\n")
		time.Sleep(time.Millisecond * 1000)
		cpu.led[i].Low() //	fmt.Printf("Off\n")
	}
	fmt.Printf("4bit CPU TD4 emulator\n")
	fmt.Printf("Reading from the serial port...\n")
	fmt.Printf("Mode: Step=%v, Speed=%5dms/inst\n", stepMode, speed)
	fmt.Printf("| PC   BP |OP-code|A register |B register |Cflag| IN port | OUT port |\n")
	fmt.Printf("|:--------|:-----:|:---------:|:---------:|:---:|:-------:|:--------:|\n")
	cpu.DumpState(cpu.PC)

	execStatus := true
	for execStatus {
		//	ステップ実行モードの場合
		if stepMode {
			fmt.Printf("> ")
			readbuffer = ""
			for { // キー入力待ち
				// PCからの受信データをチェック
				if machine.Serial.Buffered() > 0 {
					c, err := machine.Serial.ReadByte()
					if err == nil {
						if c < 32 {
							switch c {
							case '\r':
								enter_flag = true // machine.Serial.WriteByte('\r')
							case '\n':
								enter_flag = true // machine.Serial.WriteByte('\n')
							case '\b':
								if len(readbuffer) > 0 { // バックスペースで、最後尾の１文字を削除
									machine.Serial.WriteByte('\b') // 表示部分の最後の1文字を消去
									machine.Serial.WriteByte(' ')
									machine.Serial.WriteByte('\b')
									readbuffer = TrimLastChar(readbuffer) // すでに取り込んでいる文字列データの最後の1文字を消去
								}
							default:
								// println(c)	  -- >  BS=8, Enter=13
								// Convert nonprintable control characters to
								// ^A, ^B, etc.
								machine.Serial.WriteByte('^')
								machine.Serial.WriteByte(c + '@')
							}
						} else if c >= 127 {
							// Anything equal or above ASCII 127, print ^?.
							machine.Serial.WriteByte('^')
							machine.Serial.WriteByte('?')
						} else {
							// Echo the printable character back to the
							// host computer.
							machine.Serial.WriteByte(c)
							// 読み込んだ文字をエコーバックし、文字列バッファーに保存する。
							readbuffer = readbuffer + string(c)
						}
					}
				}
				// This assumes that the input is coming from a keyboard
				// so checking 120 times per second is sufficient. But if
				// the data comes from another processor, the port can
				// theoretically receive as much as 11000 bytes/second
				// (115200 baud). This delay can be removed and the
				// Serial.Read() method can be used to retrieve
				// multiple bytes from the receive buffer for each
				// iteration.
				if true == enter_flag {
					// 改行コードを検出したら、ループを抜け、次のコマンド解析に移る。
					// 同時に改行コードを出力し、次行より、実行結果を表示できるようにする。
					enter_flag = false
					fmt.Printf("\n")
					break
				}
				time.Sleep(time.Millisecond * 8)
			}
			// コマンドの解析を開始
			if len(readbuffer) > 0 {
				line := strings.Replace(readbuffer, "\t", " ", -1) // タブをスペースに置換えて、区切り文字として使えるようにする。
				line = strings.Replace(line, ",", " ", -1)
				line = strings.ToUpper(line)
				line = strings.Trim(line, " \n\r")
				elements := strings.Split(line, " ")
				firstWord := line[0]
				// コマンド解析の開始
				switch firstWord {
				/*
					実装予定
					Xコマンド	レジスタ、カウンタ、フラグ類の検査と変更
				*/
				case 'H': //	ヘルプの表示(help)
					if len(elements) == 1 {
						for i := 0; i < len(HelpText); i++ {
							fmt.Printf("%s\n", HelpText[i])
						}
					}

				case 'S': //	メモリの指定されたアドレスに値を書き込む。
					// S 0 0x30 0x01 0x02 0x04 0x08 0x40 0x90 0xF7
					// S 8 0x30 0x01 0x02 0x04 0x08 0x40 0x90 0xF7
					// S 9 0x30 0x01 0x02 0x04 0x08 0x40 0x90 0xF7
					// S 8 0x30 0x01 0x02 0x04 0x08 0x40 0x90 0xF7 0x40 0x90 0xF7
					cpu.writeMemory(elements)

				case 'B': //	ブレークポイントの参照、設定と解除
					if len(elements) == 1 {
						if (cpu.BP >= MEM_MIN) && (cpu.BP <= MEM_MAX) {
							fmt.Printf("Break point: %d\n", cpu.BP)
						} else {
							fmt.Printf("Break point: none\n")
						}
					} else if len(elements) > 1 {
						//	数値変換
						val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
						if err == nil { //	文字列=>数値変換にエラーがなければ、次のステップへ
							cpu.BP = uint8(val)
							if inRange(MEM_MIN, uint8(val), MEM_MAX) { // アドレスの範囲であれば、BPに値を設定する。
								fmt.Printf("Break point: %d\n", cpu.BP)
							} else {
								fmt.Printf("Break point: none\n")
							}
						}
					}

				case 'M': //	現在の現在のメモリ内容を表示
					if 1 == len(elements) {
						fmt.Printf("| Adress | OP-code          |\n")
						fmt.Printf("|:------:|:----------------:|\n")
						for adr := 0; adr < 16; adr++ {
							cpu.DumpMemory(uint8(adr))
						}
					}

				case 'D': //	現在のCPUのレジスタ内容を表示する。
					if 1 == len(elements) {
						cpu.DumpState(cpu.PC)
					}

				case 'T': //	レジスタ表示しながらトレース実行する回数を設定する。
					if len(elements) == 1 {
						cpu.Execute()
						cpu.DumpState(cpu.PC)
					} else if len(elements) > 1 {
						//	数値変換
						val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 64)
						if err == nil {
							//	fmt.Printf("|\n")
							loop := int(val)
							//	命令実行
							for i := 0; i < loop; i++ {
								state := cpu.Execute()
								if state != 0 {
									break //	Breakpointに到達したら、停止する。
								}
								time.Sleep(time.Duration(speed) * time.Millisecond)
								cpu.DumpState(cpu.PC)
							}
						}
					}

				case 'G': //	ユーザプログラムの連続実行
					if len(elements) == 1 {
						stepMode = false
						cpu.DumpState(cpu.PC)
					} else if len(elements) > 1 {
						//	数値変換
						val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
						if err == nil { // 文字列=>数値変換にエラーがなければ、設定速度を更新
							if inRange(MEM_MIN, uint8(val), MEM_MAX) { // アドレスの範囲であれば、PCのアドレスを更新して、連続実行モードに移行する。
								cpu.PC = uint8(val)
								stepMode = false
								cpu.DumpState(cpu.PC)
							} else {
								fmt.Printf("The address space that can be set by the program counter ranges from 0 to 15.")
								stepMode = true
							}
						} else {
							fmt.Printf("G command parameter is invalid.\n")
						}
					}

				case 'V': //	実行速度の設定(velocity)
					if len(elements) == 1 {
						fmt.Printf("Speed=%5dms/inst\n", speed)
					} else if len(elements) > 1 {
						//	数値変換
						val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 64)
						if err == nil { // 文字列=>数値変換にエラーがなければ、設定速度を更新
							speed = int(val)
							fmt.Printf("Speed=%5dms/inst\n", speed)
						} else {
							fmt.Printf("Failed to set execution speed.\n")
							fmt.Printf("Please set the execution time for one step in milliseconds.\n")
						}
					}

				case 'I': //	入力ポートの値を設定する。
					if len(elements) == 1 {
						cpu.DumpState(cpu.PC)
					} else if len(elements) > 1 {
						//	数値変換
						val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
						if err == nil {
							cpu.InPort = uint8(0x0f & val)
							cpu.DumpState(cpu.PC)
						} else {
							fmt.Printf("The input value is incorrect.")
						}
					}

				case 'Q':
					// 	fmt.Printf("\n")
					execStatus = false // プログラムを終了する。
				}
			}
		} else {
			//	通常実行モードの場合、指定時間待機
			//	命令実行
			result := cpu.Execute()
			if 0 != result {
				stepMode = true
				continue
			}
			time.Sleep(time.Duration(speed) * time.Millisecond)
			cpu.DumpState(cpu.PC)
		}
	}
	fmt.Printf("program terminated !\n")
	for {
		time.Sleep(time.Millisecond * 5000)
	}
}
