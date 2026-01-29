package main
// go fmt .\main.go
// tinygo flash -target=pico -size=short -monitor .
// tinygo build -o td4emu_tinygo.uf2 -target=pico -size=short .
// 4bitCPU td4用のエミュレータ

import (
    "machine"
    "time"
	"os"
	"bufio"
	"strconv"
	"strings"
	"fmt"

/*
	"flag"
	"log"
	"time"
*/
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
		ROM:	[16]uint8{},	// ゼロ初期化 (NOP)
		BP:		255,			// Break point 0-15以外の値は未設定の状態
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
		if line == "" {	// 空行の場合は、読み飛ばす。
			continue
		}
		if line[0] == ';' {	// コメントの場合は、読み飛ばす。
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
func (cpu *CPU) DumpMemory (adress uint8) {
	// コンソール画面をクリア（ANSIエスケープシーケンス）
	// Windowsの古いコマンドプロンプトでは効かない場合がありますが、PowerShellやVSCodeなら動作します
	// fmt.Print("\033[H\033[2J") 
	// 2進数表記のヘルパー
	bin4 := func(v uint8) string {
		return fmt.Sprintf("%04b", v & 0xF)
	}
	fmt.Printf("|   %02d   | 0x%02X 0b%s_%s |\n",
		adress, cpu.ROM[adress], bin4(cpu.ROM[adress] >> 4), bin4(cpu.ROM[adress]))
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
		return fmt.Sprintf("%04b", v & 0xF)
	}
	fmt.Printf("| PC:%02d | OP:%02X | A:%s(%X) | B:%s(%X) | C:%d | IN:%s | OUT:%s |\n",
	//	cpu.PC, cpu.ROM[cpu.PC], bin4(cpu.A), cpu.A, bin4(cpu.B), cpu.B, cInt, bin4(cpu.InPort), bin4(cpu.OutPort))
	adress, cpu.ROM[adress], bin4(cpu.A), cpu.A, bin4(cpu.B), cpu.B, cInt, bin4(cpu.InPort), bin4(cpu.OutPort))
}

// Execute 1命令実行サイクル
func (cpu *CPU) Execute() {
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
		nextPC = im // ジャンプ成立時はPCを書き換え
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

func main() {
	enter_flag := false  // 改行コードのチェック用フラグ
	var readbuffer string
    time.Sleep(time.Millisecond * 2000)
	println("4bit CPU TD4 emulator")
    println("Reading from the serial port...")
	println("| PC    |OP-code|A register |B register |Cflag| IN port | OUT port |")
	println("|:-----:|:-----:|:---------:|:---------:|:---:|:-------:|:--------:|")
	cpu := NewCPU() // TD4のオブジェクト生成および初期化
	cpu.DumpState(cpu.PC)
//	cmdStatus := false
	// loop := 1
	execStatus := true
	for execStatus {
		fmt.Printf("> ")
		readbuffer = ""
		for {  // キー入力待ち
			// PCからの受信データをチェック
			if machine.Serial.Buffered() > 0 {
				c, err := machine.Serial.ReadByte()
				if err == nil {
					if c < 32 {
						switch c {
						case '\r' : enter_flag = true // machine.Serial.WriteByte('\r')
						case '\n' : enter_flag = true // machine.Serial.WriteByte('\n')
						case '\b' : if len(readbuffer) > 0 { // バックスペースで、最後尾の１文字を削除
										machine.Serial.WriteByte('\b')  // 表示部分の最後の1文字を消去
										machine.Serial.WriteByte(' ')
										machine.Serial.WriteByte('\b')
										readbuffer = TrimLastChar(readbuffer)  // すでに取り込んでいる文字列データの最後の1文字を消去
									}
						default :
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
			//	println("true == enter_flag\n> ")
				enter_flag = false
				break
			}
			time.Sleep(time.Millisecond * 8)
		}
		// コマンドの解析を開始
		if len(readbuffer) > 0 {
			line := strings.Replace(readbuffer, "\t", " ", -1)	// タブをスペースに置換えて、区切り文字として使えるようにする。
			line = strings.ToUpper(string(line))
			elements := strings.Split(line, " ")
			firstWord := line[0]
			//	fmt.Printf("%s\n", firstWord)
			switch firstWord {
				/*
				Gコマンド	ユーザプログラムの実行
				Xコマンド	レジスタ、カウンタ、フラグ類の検査と変更
				*/
				case 'B' : //	ブレークポイントの参照、設定と解除
				if len(elements) == 1 {
					if (cpu.BP >= MEM_MIN) && (cpu.BP <= MEM_MAX) {
						fmt.Printf("\nBreak point: %d", cpu.BP)
					} else {
						fmt.Printf("\nBreak point: none")		
					}
				} else if len(elements) > 1 {
					//	数値変換
					val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
					if err == nil { // 文字列=>数値変換にエラーがなければ、次のステップへ
						if (uint8(val) >= MEM_MIN) && (uint8(val) <= MEM_MAX) { // アドレスの範囲であれば、BPに値を設定する。
							cpu.BP = uint8(val)
							fmt.Printf("\nBreak point: %d", cpu.BP)
						}
					}
				}
				println()
				case 'T' : //	レジスタ表示しながらトレース実行する回数を設定する。
				if len(elements) == 1 {
					cpu.Execute()
					cpu.DumpState(cpu.PC)	
				} else if len(elements) > 1 {
					//	数値変換
					val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
					if err == nil {
						println()
						loop := int(val)
						//	命令実行
						for i := 0; i < loop; i++ { 
							cpu.Execute()
						}
						cpu.DumpState(cpu.PC)
					}
				}
			case 'S' : //	メモリの指定されたアドレスに値を書き込む。
				if 3 == len(elements) {
					adr, err1 := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
					val, err2 := strconv.ParseInt(strings.Trim(elements[2], " \n\r"), 0, 16)
				//	fmt.Printf("\nadr:%d,val:%2x\n", adr, val)
					if err1 == nil && err2 == nil { 		//	正常に整数値に変換されたかをチェック
						if adr >= 0x00 && adr <= 0x0f {		//	指定されたアドレスがメモリ空間内であるかをチェック
							cpu.ROM[uint8(0x0f & adr)] = uint8(val)		//	メモリの指定されたアドレスの内容を書換える。
							println()
							cpu.DumpMemory(uint8(adr))
						}
					}
				}
			case 'I' : //	入力ポートの値を設定する。
				if len(elements) > 1 {
					//	数値変換
					val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
					if err == nil {
						cpu.InPort = uint8(0x0f & val)
						println()
						cpu.DumpState(cpu.PC)
					}
				}
			case 'D' : //	現在のCPUのレジスタ内容を表示する。
				if 1 == len(elements) {
					println()
					cpu.DumpState(cpu.PC)
				}
			case 'M' : //	現在の現在のメモリ内容を表示
				if 1 == len(elements) {
					println()             
					fmt.Printf("| Adress | OP-code          |\n")
					fmt.Printf("|:------:|:----------------:|\n")
					for adr := 0; adr < 16; adr++ {
						cpu.DumpMemory(uint8(adr))
					}
				}
			case 'Q' :
				println()
				execStatus = false  // プログラムを終了する。
			}
		} else {
			println()			
		}
		/*
			} else {
			// 改行のみの場合は、1ステップだけ実行する。
			cpu.Execute()
			println(" |")
			cpu.DumpState(cpu.PC)
			}
		 */
		/*	命令実行
		for i := 0; i < loop; i++ { 
			cpu.Execute()
			cpu.DumpState()
		}
		loop = 0
		*/
	}
	println("program terminated !")	
	for {
		time.Sleep(time.Millisecond * 5000)
	}
}

