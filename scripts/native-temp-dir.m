// The smoke fixture must use the same native API as the built application.
#import <Foundation/Foundation.h>
#include <stdio.h>

int main(void) {
    @autoreleasepool {
        const char *path = [NSTemporaryDirectory() UTF8String];
        if (path == NULL) return 1;
        puts(path);
    }
    return 0;
}
