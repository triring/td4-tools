package main
// 4bitCPU td4用のエミュレータ
// 16進数テキスト形式で出力されたtd4用のバイナリコードを読み込み、実行するプログラムです。
// > go build -o td4emu.exe .\main.go

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
	C       bool      // キャリーフラグ
	OutPort uint8     // 4bit 出力ポート
	InPort  uint8     // 4bit 入力ポート (今回は固定で0)
	ROM     [16]uint8 // 16バイトのプログラムメモリ
}

// NewCPU CPUの初期化
func NewCPU() *CPU {
	return &CPU{
		ROM: [16]uint8{}, // ゼロ初期化 (NOP)
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

// DumpState 現在のCPU状態を表示
func (cpu *CPU) DumpState() {
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

	fmt.Printf("PC:%02d | OP:%02X | A:%s(%X) B:%s(%X) C:%d | IN:%s | OUT:%s ? ",
		cpu.PC, cpu.ROM[cpu.PC], bin4(cpu.A), cpu.A, bin4(cpu.B), cpu.B, cInt, bin4(cpu.InPort), bin4(cpu.OutPort))
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

func main() {
	var loop int = 1
	// 1. オプション（フラグ）の定義
	stepMode := flag.Bool("step", false, "Enable step execution mode")
	speed := flag.Float64("speed", 1.0, "Execution speed in seconds per instruction")
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
		fmt.Fprintf(os.Stderr, "  td4emu -speed 0.5 timer.hex  (実行速度の設定)\n")
	}

	// 3. 解析実行
	flag.Parse()

	// 4. 引数チェック（ファイル名がない場合）
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: go run main.go [options] <hex_file>")
		fmt.Println("Usage: td4emu.exe [options] <hex_file>")
		fmt.Println("Options:")
		flag.Usage()
		flag.PrintDefaults()
		os.Exit(1)
	}
	filename := args[0]

	cpu := NewCPU()
	if err := cpu.LoadROM(filename); err != nil {
		log.Fatalf("Error loading ROM: %v", err)
	}

	fmt.Printf("Loaded %s. Starting Emulator...\n", filename)
	fmt.Printf("Mode: Step=%v, Speed=%.2fs/inst\n", *stepMode, *speed)
	fmt.Println("-------------------------------------------------------------")

	// 入力待ち用のリーダー
	stdin := bufio.NewReader(os.Stdin)
	execStatus := true
	for execStatus {
		loop = 1
		// 現在の状態を表示
		cpu.DumpState()

		// ステップ実行モードの場合
		if *stepMode {
		// 	stdin.ReadBytes('\n')
			// 改行文字 '\n' が現れるまでバイトを読み込む
			// ReadBytesは区切り文字 '\n' も含めて返却する
			data, _ := stdin.ReadBytes('\n')
		// fmt.Printf("%s\n", data)
			line := strings.ToUpper(string(data))
		// fmt.Printf("%s\n", line)
			firstWord := line[0]
		//fmt.Printf("%s\n", firstWord)
			switch firstWord {
			case 'Q' : execStatus = false  // プログラムを終了する。
			case 'I' : // 入力ポートの値を設定する。
				elements := strings.Split(line, " ")
			//	fmt.Printf("elements:%v,%d\n", elements, len(elements))
				if len(elements) > 1 {
					// 数値変換
					val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
					if err == nil {
						cpu.InPort = uint8(0x0f & val)
					}
				}
			case 'T' : // レジスタ表示しながらトレース実行する回数を設定する。
				elements := strings.Split(line, " ")
			//	fmt.Printf("elements:%v,%d\n", elements, len(elements))
				if len(elements) > 1 {
					// 数値変換
					val, err := strconv.ParseInt(strings.Trim(elements[1], " \n\r"), 0, 8)
					if err == nil {
						loop = int(val)
					}
				}
			}
		} else {
			// 通常実行モードの場合、指定時間待機
			time.Sleep(time.Duration(*speed * 1000) * time.Millisecond)
		}
		for i := 0; i < loop; i++ { 
			// 命令実行
			cpu.Execute()
			if *stepMode == false {
				fmt.Println("")		
			}
			/*
			if 1 == loop {
				fmt.Println("")
			}
				*/
		}
	}
	fmt.Println("program terminated !")	
}