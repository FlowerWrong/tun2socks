#include <stdio.h>
#include "libtun2socks.h"

// osx:   gcc -o tun2socks main.c libtun2socks.a -framework CoreFoundation -framework Security -lpthread
// linux: gcc -o tun2socks main.c libtun2socks.a -pthread
// linux: gcc -o tun2socks main.c ./libtun2socks.so
int main() {
  GoString configFile = {(char*)"/home/yy/dev/go/src/github.com/FlowerWrong/tun2socks/config.example.ini", 71};
  RunTun2socks(configFile);
  return 0;
}
