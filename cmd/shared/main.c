#include <stdio.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <pthread.h>
#include <unistd.h>
#include "libtun2socks.h"

// NOTE: cgo not support signal in c, @see https://github.com/FlowerWrong/tun2socks/issues/15

// osx:     gcc -o tun2socks main.c libtun2socks.a -framework CoreFoundation -framework Security -lpthread
// linux:   gcc -o tun2socks main.c libtun2socks.a -pthread
// windows: gcc -o tun2socks main.c libtun2socks.lib -l winmm -l ntdll -l Ws2_32

pthread_t tun2socksThreadId, uiThreadId;

void *tun2socksThread(void *arg) {
    char *configPath = (char *) arg;
    GoString configFile = {configPath, (int64_t) strlen(configPath)};
    GoStartTun2socks(configFile);
    printf("exit tun2socksThread success\n");
    return ((void *) 0);
}

void *uiThread(void *arg) {
    char *configPath = (char *) arg;
    GoString configFile = {configPath, (int64_t) strlen(configPath)};
    sleep(20);
    // GoStopTun2socks();
    GoReloadConfig(configFile);
    printf("exit uiThread success\n");
    return ((void *) 0);
}

int main(int argc, char *argv[]) {
    if (argc != 2) {
        printf("Usage: sudo ./tun2socks config.ini\n");
        return 0;
    }
    char *configPath = argv[1];

    if (pthread_create(&tun2socksThreadId, NULL, tun2socksThread, configPath)) {
        exit(EXIT_FAILURE);
    }

    if (pthread_create(&uiThreadId, NULL, uiThread, configPath)) {
        exit(EXIT_FAILURE);
    }

    pthread_join(tun2socksThreadId, NULL);
    pthread_join(uiThreadId, NULL);
    return 0;
}
