# td4-tools TD4 開発ツールセット
<!-- pandoc -f markdown -t html5 -o README.html -c github.css README.md -->

This project is a toolset created to support learning and development of the TD4, a 4-bit CPU made famous by Iku Watanami in his book [How to Create a CPU](https://book.mynavi.jp/ec/products/detail/id=22065).  
By using an assembler and emulator implemented in Go, you can develop for the TD4 even if you don't have the actual device on hand.  
Using these tools, you can easily learn programming and operating principles on your PC.

本プロジェクトは、渡波郁氏の著書 [CPUの創りかた](https://book.mynavi.jp/ec/products/detail/id=22065) で有名な4bit CPU **TD4** の学習と開発を支援するために作成したツールセットです。  
Go言語で実装された「アセンブラ」と「エミュレータ」を使用することで、実機が手元になくてもTD4の開発が可能です。  
これらを使用することで、プログラミングや動作原理をPC上で手軽に学ぶことができます。  
また、「エミュレータ」をマイコンボード上で動作するように、GoからTinyGoで書換えて、移植しました。実機での、LED制御やスイッチからの入力が可能です。

## はじめに

このプロジェクトは、以下の2つの目的のために作成したもので、実用性はありません。  
ご了承下さい。  

1. Go言語及びTinyGo言語の勉強のため
2. Googleのマルチモーダル生成AIモデル **Gemini** によるプログラミング支援機能にどれくらいの能力があるのかを検証するため

## 1. ベースとなる「TD4」について

TD4は、CPUの仕組みを理解するために設計された教育用の4bit CPUです。  
その名前は **「Tada Dousa-suru-dake-no 4bit CPU（ただ動作するだけの4bit CPU）」** に由来しており、複雑な機能を削ぎ落とし、CPUとして必要とされる最小限の機能だけで構成されています。  
極めてシンプルな構造をしており、命令セットは少なく、メモリ（ROM）容量もわずか16バイトしかありません。しかし、その制約の中で「加算」「データ転送」「条件分岐」「入出力」といったコンピュータの基礎となる動作をすべて網羅しており、CPUアーキテクチャの入門に最適です。  

TD4の基本構成  
| 呼称 | 説明           | bit数 |
|:----:|:--------------|:-----:|
| A    | 汎用レジスタ   |  4bit |
| B    | 汎用レジスタ   |  4bit |
| C    | キャリーフラグ |  1bit |
| IN   | 入力(バッファ) |  4bit |
| OUT  | 出力(バッファ) |  4bit |

本ツールセットは、このTD4の仕様に忠実に準拠しつつ、PC上での快適な開発環境を提供します。

## 2. ツール概要

本環境は、アセンブラとエミュレータの2つツールで構成されています。  
また、エミュレータには、Go言語版とマイコン上で動作するTinyGo言語版があります。  

### TD4 アセンブラ (`td4asm`)

人間が読みやすいアセンブリ言語で書かれたソースコードを、TD4が理解できる機械語（16進数コード）に変換するツールです。

#### **特徴**:

* ラベル機能対応（ジャンプ先のアドレス計算が不要です。）
* 柔軟なフォーマット（スペース、タブ区切り対応）
* リスト出力機能（`-list` オプション）
* ダンプ出力機能（`-dump` オプション）
* エミュレータ用データ出力機能（`-o` オプション）

**詳細仕様**:

* アセンブラ マニュアルへのリンク[./td4asm/README.md](./td4asm/README.md)  
* アセンブラ ソースコードへのリンク[./td4asm/main.go](./td4asm/main.go)

### TD4 エミュレータ (`td4emu`)

アセンブラで生成された機械語コードを読み込み、PCのコンソール上で実行するエミュレータです。

#### **特徴**:

* **ステップ実行モード**: 1命令ずつ停止しながらレジスタの変化を確認可能（デバッグ用）
* **内部状態の可視化**: A/Bレジスタ、キャリーフラグ、プログラムカウンタ、出力ポートの状態をリアルタイム表示
* **速度調整**: 低速から高速まで実行スピードの変更が可能

**詳細仕様**:

* エミュレータ マニュアルへのリンク [./td4emu/README.md](./td4emu/README.md)  
* エミュレータ ソースコードへのリンク[./td4emu/main.go](./td4emu/main.go)

### TinyGo版 TD4 エミュレータ (`td4emu_tinygo`)

前述のGo言語で作成したTD4 エミュレータtd4emuをマイコンボード上で動作するようにtinygoで書換えたものです。  
OSのコンソールで動いていたものを、シリアルターミナルで動くようにしただけなので、基本機能はOS上で動くGo版のtd4emuと全く変わりません。  
異なる点は、機能拡張してIN、OUT命令の入出力をマイコンボードのGPIOを制御できることです。これにより、実機でのハード制御が可能になりました。  
詳細については、以下のマニュアルをお読みください。スイッチの読み取り、LEDの制御のサンプルを掲載しています。  

* TinyGo版 エミュレータ マニュアルへのリンク [./td4emu_tinygo/README.md](./td4emu_tinygo/README.md)  
    - [TinyGo TD4 エミュレータ 汎用版](./td4emu_tinygo/core/README.md)
    - [TinyGo TD4 エミュレータ for Raspberry Pi Pico](./td4emu_tinygo/RasPiPico/README.md)
    - [TinyGo TD4 エミュレータ for Maker Pi RP2040](./td4emu_tinygo/MAKER-PI-RP2040/README.md)

## 3 必要な環境とビルド方法

本ツールはGo言語で開発されています。利用するにはGo言語の開発環境が必要です。  

1.  **Go言語環境の準備**:

    [Go言語公式サイト](https://go.dev/dl/)からインストーラーをダウンロードし、インストールしてください。

2.  **リポジトリのクローン**:

    ```bash
    git clone [https://github.com/triring/td4-tools.git](https://github.com/triring/td4-tools.git)
    cd td4-tools
    ```

3.  **ツールのビルド**:

    以下のコマンドを実行すると、ディレクトリ内に実行ファイル（`td4asm`, `td4emu` / Windowsなら `.exe`）が生成されます。
    ```bash
    # アセンブラのビルド
    go build -o td4asm td4asm/main.go

    # エミュレータのビルド
    go build -o td4emu td4emu/main.go
    ```

詳細なビルド方法については、それぞれのツールのソースコードが置かれているディレクトリ内のREADME.mdをお読み下さい。  

## 4. 開発ワークフロー（組み合わせて使う方法）

これら2つのツールを組み合わせることで、以下のようなサイクルで開発を行うことができます。

### 手順例

**Step 1: ソースコードの作成**

テキストエディタでアセンブリコードを作成します（例: `3ByteBlink.td4`）。

```assembly
; LED点滅プログラム
START:
    OUT 15     ; LED全点灯
    OUT 0      ; 全消灯
    JMP START  ; 繰り返し
```

**Step 2: アセンブル (機械語への変換)**

アセンブラを使い、ソースコードを機械語データ(`3ByteBlink.hex`)に変換します。  
`-o` オプションを使うことで、エミュレータですぐに読める形式で保存されます。

```bash
./td4asm -o 3ByteBlink.hex 3ByteBlink.td4
```

**Step 3: エミュレーション (動作確認)**

生成されたデータをエミュレータに読み込ませて実行します。
最初は `-step` オプションを付けて、1行ずつ挙動を確認するのがおすすめです。

```bash
./td4emu -step 3ByteBlink.hex
```

画面には以下のようにCPUの内部状態が表示され、意図通りに動いているか確認できます。  

```text
-------------------------------------------------------------
PC:00 | OP:BF | A:0000(0) B:0000(0) C:0 | IN:0000 | OUT:0000 ?
PC:01 | OP:B0 | A:0000(0) B:0000(0) C:0 | IN:0000 | OUT:1111 ?
PC:02 | OP:F0 | A:0000(0) B:0000(0) C:0 | IN:0000 | OUT:0000 ?
PC:00 | OP:BF | A:0000(0) B:0000(0) C:0 | IN:0000 | OUT:0000 ?
PC:01 | OP:B0 | A:0000(0) B:0000(0) C:0 | IN:0000 | OUT:1111 ?
PC:02 | OP:F0 | A:0000(0) B:0000(0) C:0 | IN:0000 | OUT:0000 ?
PC:00 | OP:BF | A:0000(0) B:0000(0) C:0 | IN:0000 | OUT:0000 ?
PC:01 | OP:B0 | A:0000(0) B:0000(0) C:0 | IN:0000 | OUT:1111 ?
PC:02 | OP:F0 | A:0000(0) B:0000(0) C:0 | IN:0000 | OUT:0000 ? 
...

```

* 詳細な仕様やコマンドオプションについては、各ツールのドキュメントを参照してください。  

## 5. サンプルコード

以下のディレクトリでサンプルコードを公開しています。

./samples  

- [./samples/3ByteBlink.td4](./samples/3ByteBlink.td4)  
    わずか3byteの命令でLチカができます。
- [./samples/AddOne.td4](./samples/AddOne.td4)  
    入力ポートの値に1を足して出力ポートへ送り出します。
- [./samples/Brink.td4](./samples/Brink.td4)  
    シンプルなLチカです。
- [./samples/InOut.td4](./samples/InOut.td4)  
    入力ポートの内容をそのまま出力ポートに送ります。
- [./samples/KnightRider.td4](./samples/KnightRider.td4)  
    LEDが左から右へ、右から左へと流れるように点灯します。
- [./samples/Summation.td4](./samples/Summation.td4)  
    1+2+4+8を計算し、結果を出力ポートに送ります。
- [./samples/Timer.td4](./samples/Timer.td4)  
    15から0までカウントダウンし、0になったらLEDが点滅します。

------------