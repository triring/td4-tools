# TD4 エミュレータ by tinygo
<!-- pandoc -f markdown -t html5 -o README.html -c github.css README.md -->

## 1. 概要

Go言語で作成したtd4emuをマイコンボード上でも使えるようにtinygoで書き換え移植したものです。　　
OSのコンソールで動いていたものを、シリアルターミナルで動くようにしただけなので、一般のOS上で動くGo版のtd4emuと全く変わりません。
Raspberry Pi Pico で動作確認を行っています。

## 2. コンパイルと実行プログラムの書込み方法

ソースコード[main.go](./main.go)があるディレクトリに移動して、以下のコマンドを実行して下さい。  

```bash
> tinygo flash -target=pico -size=short -monitor .
   code    data     bss |   flash     ram
  75464    1596    5200 |   77060    6796
Connected to COM4. Press Ctrl-C to exit.
4bit CPU TD4 emulator
Reading from the serial port...
Mode: Step=true, Speed= 1000ms/inst
| PC   BP |OP-code|A register |B register |Cflag| IN port | OUT port |
|:--------|:-----:|:---------:|:---------:|:---:|:-------:|:--------:|
| PC:00   | OP:00 | A:0000(0) | B:0000(0) | C:0 | IN:0000 | OUT:0000 |
>
```

上記は、Raspi pico用のコンパイルと実行プログラムの書込み方法です。  
他のマイコンボードを利用する場合は、以下のコマンドを実行して下さい。  
表示されるtinygoで利用可能なマイコンボードの一覧が表示されるので、この中から使用するマイコンボード名を探して、**-target**オプションで指定して下さい。  

```bash
> tinygo targets
adafruit-esp32-feather-v2
ae-rp2040
arduino
arduino-leonardo
arduino-mega1280
arduino-mega2560
arduino-mkr1000
arduino-mkrwifi1010
arduino-nano
arduino-nano-new
arduino-nano33
arduino-zero
atmega1284p
atsame54-xpro
...
...

```

## 3. 使用方法について

[../README.md](../README.md) の操作方法と基本コマンドの使い方をお読み下さい。
