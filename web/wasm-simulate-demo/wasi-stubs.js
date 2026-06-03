/**
 * Go 1.26+ wasip1 links sock_accept (and related) imports that browser_wasi_shim omits.
 * Wasm requires every import to be a function at instantiate time.
 */

const ERRNO_NOTSUP = 58;
const ERRNO_BADF = 8;
const ERRNO_INVAL = 28;

/**
 * @param {WASI} wasi
 */
export function patchWasiForGoWasip1(wasi) {
    const imp = wasi.wasiImport;
    if (!imp) {
        return;
    }

    const errno = () => ERRNO_NOTSUP;

    const socketStubs = {
        sock_accept: errno,
        sock_open: errno,
        sock_bind: errno,
        sock_listen: errno,
        sock_connect: errno,
    };

    for (const [name, fn] of Object.entries(socketStubs)) {
        if (typeof imp[name] !== "function") {
            imp[name] = fn;
        }
    }

    // Replace throw-based stubs with errno returns so guest code can handle errors.
    imp.sock_recv = function sock_recv() {
        return ERRNO_NOTSUP;
    };
    imp.sock_send = function sock_send() {
        return ERRNO_NOTSUP;
    };
    imp.sock_shutdown = function sock_shutdown(fd, how) {
        if (typeof imp.fd_close === "function") {
            return imp.fd_close(fd);
        }
        return ERRNO_BADF;
    };
}
