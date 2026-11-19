package main

import _ "embed"

//go:embed assets/mediainfo-amd64.dll.gz
var compressedMediaDLL []byte
