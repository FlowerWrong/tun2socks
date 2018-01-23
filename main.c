#include <stdio.h>
#include "libtun2socks.h"

// gcc -o hello main.c libtun2socks.a -framework CoreFoundation -framework Security -lpthread
int main() {
  GoString configFile = {(char*)"/Users/kingyang/go/src/github.com/FlowerWrong/tun2socks/config.example.ini", 74};
  RunTun2socks(configFile);
  return 0;
}
