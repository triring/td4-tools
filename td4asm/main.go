package main

// 4bitCPU td4用のアセンブラ
// td4用のソースコードを読み込み、アセンブルして、16進数テキスト形式に変換して出力するプログラムです。
// > go fmt .\main.go
// > go build -o td4asm.exe .\main.go

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// InstructionSet TD4の命令セット定義
var InstructionSet = map[string]bool{
	"ADD": true, "MOV": true, "JMP": true, "JNC": true,
	"IN": true, "OUT": true, "NOP": true,
}

// SymbolTable ラベルとアドレスの対応表
type SymbolTable map[string]int

// Assembler アセンブラ構造体
type Assembler struct {
	lines       []string
	symbolTable SymbolTable
	binaries    []uint8
	debugLines  []string // バイナリに対応するソースコード表示用
}

// NewAssembler ソースコードの行スライスを受け取る
func NewAssembler(lines []string) *Assembler {
	return &Assembler{
		lines:       lines,
		symbolTable: make(SymbolTable),
		binaries:    make([]uint8, 0),
		debugLines:  make([]string, 0),
	}
}

// CleanLine コメント除去と空白の正規化を行い、トークン（単語）のリストを返す
func (asm *Assembler) CleanLine(line string) []string {
	// 1. コメント(;)以降を削除
	if idx := strings.Index(line, ";"); idx != -1 {
		line = line[:idx]
	}
	// 2. カンマをスペースに置換
	line = strings.ReplaceAll(line, ",", " ")

	// 3. 空白で分割
	fields := strings.Fields(line)
	return fields
}

// Pass1 ラベルのアドレスを解決する
func (asm *Assembler) Pass1() error {
	pc := 0
	for _, line := range asm.lines {
		tokens := asm.CleanLine(line)
		if len(tokens) == 0 {
			continue
		}

		firstWord := strings.ToUpper(tokens[0])

		if _, isInst := InstructionSet[firstWord]; !isInst {
			// ラベル定義
			labelName := strings.TrimSuffix(firstWord, ":")
			if _, exists := asm.symbolTable[labelName]; exists {
				return fmt.Errorf("duplicate label: %s", labelName)
			}
			asm.symbolTable[labelName] = pc
			// ラベルの後に命令が続いている場合 (例: "LOOP: MOV A, 1")の処理
			// ラベルのみの行の場合は、PCをインクリメントしない。
			if len(tokens) > 1 {
				pc++
			}
		} else {
			// 命令のみ
			pc++
		}
	}
	return nil
}

// Pass2 機械語を生成し、表示用文字列も保存する
func (asm *Assembler) Pass2() error {
	pc := 0
	for lineNum, line := range asm.lines {
		tokens := asm.CleanLine(line)
		if len(tokens) == 0 {
			continue
		}

		var mnemonic string
		var args []string

		firstWord := strings.ToUpper(tokens[0])
		if _, isInst := InstructionSet[firstWord]; !isInst {
			// ラベル行
			if len(tokens) == 1 {
				continue
			}
			// ラベル + 命令
			mnemonic = strings.ToUpper(tokens[1])
			args = tokens[2:]
		} else {
			// 命令のみ
			mnemonic = strings.ToUpper(tokens[0])
			args = tokens[1:]
		}

		// 機械語生成
		code, err := asm.generateCode(mnemonic, args, pc)
		if err != nil {
			return fmt.Errorf("line %d: %v", lineNum+1, err)
		}

		// 結果を保存
		asm.binaries = append(asm.binaries, code)

		// 表示用に整形したソースコードを保存 (例: "MOV A, B")
		// 引数の間にカンマを入れて読みやすくする
		prettyArgs := strings.Join(args, ", ")
		asm.debugLines = append(asm.debugLines, fmt.Sprintf("%s %s", mnemonic, prettyArgs))

		pc++
	}
	return nil
}

// generateCode 命令と引数からバイナリ(1byte)を生成
func (asm *Assembler) generateCode(mnemonic string, args []string, currentPC int) (uint8, error) {
	parseImm := func(s string) (uint8, error) {
		// ラベル解決
		if val, ok := asm.symbolTable[strings.ToUpper(s)]; ok {
			return uint8(val & 0x0F), nil
		}
		// 数値変換
		val, err := strconv.ParseInt(s, 0, 8)
		if err != nil {
			return 0, fmt.Errorf("invalid immediate or label: %s", s)
		}
		if val < 0 || val > 15 {
			return 0, fmt.Errorf("immediate out of range (0-15): %d", val)
		}
		return uint8(val), nil
	}

	switch mnemonic {
	case "ADD": // レジスタに即値を加算
		if len(args) != 2 {
			return 0, fmt.Errorf("ADD requires 2 arguments")
		}
		im, err := parseImm(args[1])
		if err != nil {
			return 0, err
		}
		if args[0] == "A" {
			return 0x00 | im, nil
		}
		if args[0] == "B" {
			return 0x50 | im, nil
		}
		return 0, fmt.Errorf("ADD target must be A or B")

	case "MOV": // レジスタの内容を変更
		if len(args) != 2 {
			return 0, fmt.Errorf("MOV requires 2 arguments")
		}
		target, src := args[0], args[1]
		if target == "A" && src == "B" {
			return 0x10, nil
		} // AレジスタにBレジスタの内容を転送
		if target == "B" && src == "A" {
			return 0x40, nil
		} // BレジスタにAレジスタの内容を転送
		if target == "A" { // Aレジスタに即値を代入
			im, err := parseImm(src)
			if err != nil {
				return 0, err
			}
			return 0x30 | im, nil
		}
		if target == "B" { // Bレジスタに即値を代入
			im, err := parseImm(src)
			if err != nil {
				return 0, err
			}
			return 0x70 | im, nil
		}
		return 0, fmt.Errorf("invalid MOV operands")

	case "JMP": // 指定アドレスへジャンプ
		if len(args) != 1 {
			return 0, fmt.Errorf("JMP requires 1 argument")
		}
		im, err := parseImm(args[0])
		if err != nil {
			return 0, err
		}
		return 0xF0 | im, nil

	case "JNC": // Cフラグが0なら指定アドレスへジャンプ
		if len(args) != 1 {
			return 0, fmt.Errorf("JNC requires 1 argument")
		}
		im, err := parseImm(args[0])
		if err != nil {
			return 0, err
		}
		return 0xE0 | im, nil

	case "IN": // 入力
		if len(args) != 1 {
			return 0, fmt.Errorf("IN requires 1 argument")
		}
		if args[0] == "A" {
			return 0x20, nil
		} // Aレジスタに入力ポートの内容を転送
		if args[0] == "B" {
			return 0x60, nil
		} // Bレジスタに入力ポートの内容を転送
		return 0, fmt.Errorf("IN target must be A or B")

	case "OUT": // 出力
		if len(args) != 1 {
			return 0, fmt.Errorf("OUT requires 1 argument")
		}
		if args[0] == "B" {
			return 0x90, nil
		} // Bレジスタの内容を出力ポートへ転送
		im, err := parseImm(args[0])
		if err != nil {
			return 0, err
		}
		return 0xB0 | im, nil // 即値を出力ポートへ転送

	case "NOP": // 何もしない
		return 0x00, nil
	}

	return 0, fmt.Errorf("unknown instruction: %s", mnemonic)
}

func main() {
	var noOption bool = false // オプションの指定がない場合のフラグ

	// 1. オプション（フラグ）の定義
	var dumpFlag bool
	flag.BoolVar(&dumpFlag, "dump", false, "アセンブル結果を16進数ダンプ形式で表示する")
	var listFlag bool
	flag.BoolVar(&listFlag, "list", false, "詳細なアセンブル情報を表示する")
	var outputFile string
	flag.StringVar(&outputFile, "o", "", "アセンブル結果を16進数ダンプ形式でファイルに保存する")

	// 2. ヘルプ表示のカスタマイズ
	flag.Usage = func() {
		// ヘルプのヘッダー部分
		fmt.Fprintf(os.Stderr, "TD4 アセンブラ\n")
		fmt.Fprintf(os.Stderr, "4bit CPU td4用のアセンブラです。\n\n")
		// 使い方の構文
		fmt.Fprintf(os.Stderr, "使い方:\n")
		fmt.Fprintf(os.Stderr, "td4asm [オプション] ファイル名\n\n")

		// オプション一覧（定義したフラグを自動で表示してくれる便利な関数）
		fmt.Fprintf(os.Stderr, "オプション:\n")
		flag.PrintDefaults()
		// 使用例
		fmt.Fprintf(os.Stderr, "\n使用例:\n")
		fmt.Fprintf(os.Stderr, "  td4asm Sample.td4                (アセンブル結果のみを出力)\n")
		fmt.Fprintf(os.Stderr, "  td4asm -dump Sample.td4          (DUMP形式で出力)\n")
		fmt.Fprintf(os.Stderr, "  td4asm -list Sample.td4          (LIST形式で出力)\n")
		fmt.Fprintf(os.Stderr, "  td4asm -o Sample.hex Sample.td4  (HEX形式でファイルに保存)\n")
		fmt.Fprintf(os.Stderr, "  td4asm -help                     (ヘルプの表示)\n")
	}

	// 3. 解析実行
	flag.Parse()

	// 4. 引数チェック（ファイル名がない場合）
	args := flag.Args()
	if len(args) < 1 {
		// エラー時にもヘルプを表示してあげるのが親切です
		flag.Usage()
		os.Exit(1)
	}

	// Hex ファイルの読み込み
	filePath := args[0]
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading file: %v", err)
		os.Exit(1)
	}

	// オプションの指定がない場合のフラグを立てる。
	if dumpFlag == false && listFlag == false && outputFile == "" {
		noOption = true
	}
	asm := NewAssembler(lines)
	// fmt.Printf("Assembling %s ...\n", filePath)

	if err := asm.Pass1(); err != nil {
		log.Fatalf("Pass 1 Error: %v", err)
		os.Exit(1)
	} else {
		if noOption {
			fmt.Println("Pass 1 : Ok!")
		}
	}

	if err := asm.Pass2(); err != nil {
		log.Fatalf("Pass 2 Error: %v", err)
		os.Exit(2)
	} else {
		if noOption {
			fmt.Println("Pass 2 : Ok!")
		}
	}
	if noOption == true {
		fmt.Printf("Assembly completed without errors.\nCode size %d bytes.\n", len(asm.binaries))
		os.Exit(0)
	}

	// 結果をHex形式でダンプ
	if dumpFlag {
		for _, b := range asm.binaries {
			fmt.Printf("%02X\n", b)
		}
	}

	// 結果をリスト表示
	if listFlag {
		// テーブル形式で出力
		fmt.Println("\n ADDR      | BINARY    | HEX | SOURCE CODE")
		fmt.Println("-----------|-----------|-----|----------------")
		for i, b := range asm.binaries {
			// debugLinesスライスから対応するソース文字列を取得
			sourceCode := ""
			if i < len(asm.debugLines) {
				sourceCode = asm.debugLines[i]
			}
			fmt.Printf(" %02X [%04b] | %04b_%04b |  %02X | %s\n", i, i, b>>4, b&0x0f, b, sourceCode)
		}
		fmt.Printf("\nSuccess! Generated %d bytes.\n", len(asm.binaries))
	}

	// アセンブル結果をHEX形式でファイルに保存
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer f.Close()

		writer := bufio.NewWriter(f)
		_, err = fmt.Fprintf(writer, "S 0x00 ")
		if err != nil {
			log.Fatalf("Error writing to file: %v", err)
		}
		for _, b := range asm.binaries {
			// エミュレータが読み込める形式（HEX文字列＋改行）で書き込む
			_, err := fmt.Fprintf(writer, "0x%02X ", b)
			if err != nil {
				log.Fatalf("Error writing to file: %v", err)
			}
		}
		writer.Flush()
		fmt.Printf("Output saved to '%s'\n", outputFile)
	}
	os.Exit(0)
}
