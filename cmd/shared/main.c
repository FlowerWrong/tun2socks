#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include "libtun2socks.h"

// osx:     gcc -o tun2socks main.c libtun2socks.a -framework CoreFoundation -framework Security -lpthread
// linux:   gcc -o tun2socks main.c libtun2socks.a -pthread
// windows: gcc -o tun2socks main.c libtun2socks.lib -l winmm -l ntdll -l Ws2_32
int main(int argc, char *argv[]) {
  if (argc != 2) {
    printf("Usage: sudo ./tun2socks config.ini\n");
    return 0;
  }
  char * configPath = argv[1];
  GoString configFile = {configPath, (int64_t)strlen(configPath)};
  RunTun2socks(configFile);
  return 0;
}
