//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
#import <dispatch/dispatch.h>
#include <stdint.h>
#include <stdatomic.h>
#include <pthread.h>
static _Atomic uint64_t fairdropAlertEpoch;
static _Atomic bool fairdropAlertReady;
static _Atomic bool fairdropAlertActive;
static _Atomic uint64_t fairdropAlertGeneration;
static _Atomic bool fairdropAlertStartQueued;
static pthread_mutex_t fairdropAlertQueueLock = PTHREAD_MUTEX_INITIALIZER;
static bool fairdropAlertQueued;
static uint64_t fairdropPendingEpoch;
static uint64_t fairdropPendingFocus;
static _Atomic bool fairdropAlertTestMode;
static _Atomic bool fairdropAlertTestHold;
static _Atomic int fairdropAlertTestCalls;
static void fairdropInvokeAttention(NSRequestUserAttentionType kind) {
    if (atomic_load(&fairdropAlertTestMode)) {
        if (kind == NSInformationalRequest) atomic_fetch_add(&fairdropAlertTestCalls, 1);
    } else {
        [NSApp requestUserAttention:kind];
    }
}
static id fairdropActiveObserver;
static id fairdropInactiveObserver;
static void fairdropAlertStart(void) {
    bool expected = false;
    if (!atomic_compare_exchange_strong(&fairdropAlertStartQueued, &expected, true)) return;
    dispatch_async(dispatch_get_main_queue(), ^{
        if (NSApp == nil) return;
        fairdropActiveObserver = [[NSNotificationCenter defaultCenter]
            addObserverForName:NSApplicationDidBecomeActiveNotification object:NSApp queue:nil
            usingBlock:^(NSNotification *note) {
                atomic_store(&fairdropAlertActive, true);
                atomic_fetch_add(&fairdropAlertGeneration, 1);
            }];
        fairdropInactiveObserver = [[NSNotificationCenter defaultCenter]
            addObserverForName:NSApplicationDidResignActiveNotification object:NSApp queue:nil
            usingBlock:^(NSNotification *note) {
                atomic_store(&fairdropAlertActive, false);
                atomic_fetch_add(&fairdropAlertGeneration, 1);
            }];
        atomic_store(&fairdropAlertActive, [NSApp isActive]);
        atomic_store(&fairdropAlertGeneration, 1);
        atomic_store(&fairdropAlertReady, true);
    });
}
static uint64_t fairdropAlertSnapshot(void) {
    if (!atomic_load(&fairdropAlertReady)) return 0;
    uint64_t generation = atomic_load(&fairdropAlertGeneration);
    return (generation << 1) | (!atomic_load(&fairdropAlertActive) ? 1 : 0);
}
static void fairdropAlertInvalidate(uint64_t epoch) {
    atomic_store(&fairdropAlertEpoch, epoch);
}
static bool fairdropAlertCanRequest(uint64_t epoch, uint64_t focus, bool available, bool active) {
    return available && !active && atomic_load(&fairdropAlertReady) &&
        atomic_load(&fairdropAlertEpoch) == epoch &&
        atomic_load(&fairdropAlertGeneration) == focus;
}
static void fairdropAlertDeliver(void) {
    pthread_mutex_lock(&fairdropAlertQueueLock);
    uint64_t latestEpoch = fairdropPendingEpoch;
    uint64_t latestFocus = fairdropPendingFocus;
    fairdropAlertQueued = false;
    pthread_mutex_unlock(&fairdropAlertQueueLock);
    bool test = atomic_load(&fairdropAlertTestMode);
    bool available = test || NSApp != nil;
    bool active = test ? atomic_load(&fairdropAlertActive) : (NSApp != nil && [NSApp isActive]);
    if (fairdropAlertCanRequest(latestEpoch, latestFocus, available, active)) {
        fairdropInvokeAttention(NSInformationalRequest);
    }
}
static void fairdropAlertRequest(uint64_t epoch, uint64_t focus) {
    pthread_mutex_lock(&fairdropAlertQueueLock);
    fairdropPendingEpoch = epoch;
    fairdropPendingFocus = focus;
    if (fairdropAlertQueued) { pthread_mutex_unlock(&fairdropAlertQueueLock); return; }
    fairdropAlertQueued = true;
    pthread_mutex_unlock(&fairdropAlertQueueLock);
    if (atomic_load(&fairdropAlertTestMode)) {
        if (!atomic_load(&fairdropAlertTestHold)) fairdropAlertDeliver();
        return;
    }
    dispatch_async(dispatch_get_main_queue(), ^{
        @autoreleasepool {
            fairdropAlertDeliver();
        }
    });
}
static void fairdropAlertTestBegin(void) {
    atomic_store(&fairdropAlertTestMode, true);
    atomic_store(&fairdropAlertTestHold, false);
    atomic_store(&fairdropAlertTestCalls, 0);
    atomic_store(&fairdropAlertReady, true);
    atomic_store(&fairdropAlertActive, false);
    atomic_store(&fairdropAlertGeneration, 1);
    pthread_mutex_lock(&fairdropAlertQueueLock);
    fairdropAlertQueued = false;
    pthread_mutex_unlock(&fairdropAlertQueueLock);
}
static void fairdropAlertTestEnd(void) {
    atomic_store(&fairdropAlertTestMode, false);
    atomic_store(&fairdropAlertReady, false);
}
static void fairdropAlertTestFocus(bool active) {
    atomic_store(&fairdropAlertActive, active);
    atomic_fetch_add(&fairdropAlertGeneration, 1);
}
static void fairdropAlertTestHoldQueue(bool hold) { atomic_store(&fairdropAlertTestHold, hold); }
static void fairdropAlertTestFlush(void) { fairdropAlertDeliver(); }
static int fairdropAlertTestCount(void) { return atomic_load(&fairdropAlertTestCalls); }
*/
import "C"

func nativeCompletionCue(request alertRequest) {
	C.fairdropAlertRequest(C.uint64_t(request.epoch), C.uint64_t(request.focus))
}
func nativeCompletionInvalidate(epoch uint64) { C.fairdropAlertInvalidate(C.uint64_t(epoch)) }
func nativeCompletionStart()                  { C.fairdropAlertStart() }
func nativeCompletionEnable()                 {}
func nativeCompletionStop()                   {}
func nativeCompletionSnapshot() focusSnapshot {
	bits := uint64(C.fairdropAlertSnapshot())
	return focusSnapshot{generation: bits >> 1, background: bits&1 == 1}
}
func nativeAlertTestBegin()            { C.fairdropAlertTestBegin() }
func nativeAlertTestEnd()              { C.fairdropAlertTestEnd() }
func nativeAlertTestFocus(active bool) { C.fairdropAlertTestFocus(C.bool(active)) }
func nativeAlertTestHold(hold bool)    { C.fairdropAlertTestHoldQueue(C.bool(hold)) }
func nativeAlertTestFlush()            { C.fairdropAlertTestFlush() }
func nativeAlertTestCount() int        { return int(C.fairdropAlertTestCount()) }
