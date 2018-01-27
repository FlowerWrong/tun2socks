#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include <signal.h>
#include "libtun2socks.h"

// osx:     gcc -o tun2socks main.c libtun2socks.a -framework CoreFoundation -framework Security -lpthread
// linux:   gcc -o tun2socks main.c libtun2socks.a -pthread
// windows: gcc -o tun2socks main.c libtun2socks.lib -l winmm -l ntdll -l Ws2_32

void intHandler(int sig) {
    printf("stop tun2socks %d\n", sig);
    StopTun2socks();
}

int main(int argc, char *argv[]) {
    if (argc != 2) {
        printf("Usage: sudo ./tun2socks config.ini\n");
        return 0;
    }


    signal(SIGINT, intHandler);

    char *configPath = argv[1];
    GoString configFile = {configPath, (int64_t) strlen(configPath)};
    StartTun2socks(configFile);
    return 0;
}
