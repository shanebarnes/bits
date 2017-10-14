// Adapted source code from:
// https://msdn.microsoft.com/en-us/library/windows/desktop/ms737591(v=vs.85).aspx
// https://msdn.microsoft.com/en-us/library/windows/desktop/ms737593(v=vs.85).aspx
// https://blogs.technet.microsoft.com/wincat/2012/12/05/fast-tcp-loopback-performance-and-low-latency-with-windows-server-2012-tcp-loopback-fast-path/

#define WIN32_LEAN_AND_MEAN

#include <windows.h>
#include <winsock2.h>
#include <ws2tcpip.h>
#include <mstcpip.h>
#include <stdlib.h>
#include <stdio.h>

#include <algorithm>
#include <chrono>
#include <string>

#pragma comment (lib, "Ws2_32.lib")
#pragma comment (lib, "Mswsock.lib")
#pragma comment (lib, "AdvApi32.lib")

#define DEFAULT_BUFLEN 131072

std::string GetErrorMessage()
{
    std::string res("");
    DWORD err(::GetLastError());

    if (err != 0) {
        LPSTR buf(nullptr);
        DWORD flags(FORMAT_MESSAGE_ALLOCATE_BUFFER | FORMAT_MESSAGE_FROM_SYSTEM | FORMAT_MESSAGE_IGNORE_INSERTS);
        size_t size(FormatMessageA(flags, NULL, err, MAKELANGID(LANG_NEUTRAL, SUBLANG_DEFAULT), (LPSTR)&buf, 0, NULL));

        res = std::string(buf, size);
        LocalFree(buf);
    }

    return res;
}

void setTcpFastPath(SOCKET socket) {
    int optVal(1);
    DWORD optLen(0);

    int status(WSAIoctl(socket, SIO_LOOPBACK_FAST_PATH, &optVal, sizeof(optVal), NULL, 0, &optLen, 0, 0));

    if (status == SOCKET_ERROR) {
        fprintf(stderr, "TCP fast path option could not be set: %s\n", GetErrorMessage().c_str());
    }
    else {
        fprintf(stderr, "Enabled TCP fast path\n");
    }
}

int clientSend(const std::string &addr, const std::string &port, const int32_t sendLimitMb, const bool tcpFastPath) {
    WSADATA wsaData;
    SOCKET ConnectSocket = INVALID_SOCKET;
    struct addrinfo *result = NULL,
        *ptr = NULL,
        hints;
    char sendbuf[DEFAULT_BUFLEN];
    int sendbuflen = DEFAULT_BUFLEN;
    int iResult;

    fprintf(stderr, "Sending up to %d MB\n", sendLimitMb);

    // Initialize Winsock
    iResult = WSAStartup(MAKEWORD(2, 2), &wsaData);
    if (iResult != 0) {
        printf("WSAStartup failed with error: %d\n", iResult);
        return 1;
    }

    ZeroMemory(&hints, sizeof(hints));
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_STREAM;
    hints.ai_protocol = IPPROTO_TCP;

    // Resolve the server address and port
    iResult = getaddrinfo(addr.c_str(), port.c_str(), &hints, &result);
    if (iResult != 0) {
        printf("getaddrinfo failed with error: %d\n", iResult);
        WSACleanup();
        return 1;
    }

    // Attempt to connect to an address until one succeeds
    for (ptr = result; ptr != NULL; ptr = ptr->ai_next) {

        // Create a SOCKET for connecting to server
        ConnectSocket = socket(ptr->ai_family, ptr->ai_socktype,
            ptr->ai_protocol);
        if (ConnectSocket == INVALID_SOCKET) {
            printf("socket failed with error: %ld\n", WSAGetLastError());
            WSACleanup();
            return 1;
        }

        if (tcpFastPath) {
            setTcpFastPath(ConnectSocket);
        }

        // Connect to server.
        iResult = connect(ConnectSocket, ptr->ai_addr, (int)ptr->ai_addrlen);
        if (iResult == SOCKET_ERROR) {
            closesocket(ConnectSocket);
            ConnectSocket = INVALID_SOCKET;
            continue;
        }
        break;
    }

    freeaddrinfo(result);

    if (ConnectSocket == INVALID_SOCKET) {
        printf("Unable to connect to %s\n", addr.c_str());
        WSACleanup();
        return 1;
    }

    auto startTime(std::chrono::system_clock::now());

    // Send an initial buffer
    int64_t totalSendBytes = 0;
    do {
        iResult = send(ConnectSocket, sendbuf, sendbuflen, 0);
        if (iResult == SOCKET_ERROR) {
            printf("send failed with error: %d\n", WSAGetLastError());
            break;
        }
        else {
            totalSendBytes += iResult;
        }
    } while (totalSendBytes < static_cast<int64_t>(sendLimitMb) * 1000000);

    auto stopTime(std::chrono::system_clock::now());
    auto delta(stopTime - startTime);
    auto durationMs(std::chrono::duration_cast<std::chrono::milliseconds>(delta).count());

    fprintf(stderr,
        "Send bytes / time / rate: %.3f MB / %lld ms / %lld Mbps\n",
        static_cast<double>(totalSendBytes) / 1000000.,
        durationMs,
        durationMs == 0 ? 0 : totalSendBytes * 8 / (durationMs * 1000));

    // cleanup
    closesocket(ConnectSocket);
    WSACleanup();

    return 0;
}

int serverRecv(const std::string &addr, const std::string &port, const int32_t recvLimitMb, const bool tcpFastPath) {
    WSADATA wsaData;
    int iResult;

    SOCKET ListenSocket = INVALID_SOCKET;
    SOCKET ClientSocket = INVALID_SOCKET;

    struct addrinfo *result = NULL;
    struct addrinfo hints;

    char recvbuf[DEFAULT_BUFLEN];
    int recvbuflen = DEFAULT_BUFLEN;

    fprintf(stderr, "Receiving up to %d MB\n", recvLimitMb);

    // Initialize Winsock
    iResult = WSAStartup(MAKEWORD(2, 2), &wsaData);
    if (iResult != 0) {
        printf("WSAStartup failed with error: %d\n", iResult);
        return 1;
    }

    ZeroMemory(&hints, sizeof(hints));
    hints.ai_family = AF_INET;
    hints.ai_socktype = SOCK_STREAM;
    hints.ai_protocol = IPPROTO_TCP;
    hints.ai_flags = AI_PASSIVE;

    // Resolve the server address and port
    iResult = getaddrinfo(addr.c_str(), port.c_str(), &hints, &result);
    if (iResult != 0) {
        printf("getaddrinfo failed with error: %d\n", iResult);
        WSACleanup();
        return 1;
    }

    // Create a SOCKET for connecting to server
    ListenSocket = socket(result->ai_family, result->ai_socktype, result->ai_protocol);
    if (ListenSocket == INVALID_SOCKET) {
        printf("socket failed with error: %ld\n", WSAGetLastError());
        freeaddrinfo(result);
        WSACleanup();
        return 1;
    }

    // Setup the TCP listening socket
    iResult = bind(ListenSocket, result->ai_addr, (int)result->ai_addrlen);
    if (iResult == SOCKET_ERROR) {
        printf("bind failed with error: %d\n", WSAGetLastError());
        freeaddrinfo(result);
        closesocket(ListenSocket);
        WSACleanup();
        return 1;
    }

    freeaddrinfo(result);

    iResult = listen(ListenSocket, SOMAXCONN);
    if (iResult == SOCKET_ERROR) {
        printf("listen failed with error: %d\n", WSAGetLastError());
        closesocket(ListenSocket);
        WSACleanup();
        return 1;
    }

    if (tcpFastPath) {
        setTcpFastPath(ListenSocket);
    }

    // Accept a client socket
    ClientSocket = accept(ListenSocket, NULL, NULL);
    if (ClientSocket == INVALID_SOCKET) {
        printf("accept failed with error: %d\n", WSAGetLastError());
        closesocket(ListenSocket);
        WSACleanup();
        return 1;
    }

    auto startTime(std::chrono::system_clock::now());

    // No longer need server socket
    closesocket(ListenSocket);

    // Receive until the peer shuts down the connection
    int64_t totalRecvBytes = 0;
    do {
        iResult = recv(ClientSocket, recvbuf, recvbuflen, 0);
        if (iResult > 0) {
            totalRecvBytes += iResult;
        } else if (iResult == 0) {
            printf("Connection closing...\n");
            break;
        } else {
            printf("recv failed with error: %d\n", WSAGetLastError());
            closesocket(ClientSocket);
            WSACleanup();
            return 1;
        }

    } while (totalRecvBytes < static_cast<int64_t>(recvLimitMb) * 1000000);

    auto stopTime(std::chrono::system_clock::now());
    auto delta(stopTime - startTime);
    auto durationMs(std::chrono::duration_cast<std::chrono::milliseconds>(delta).count());

    fprintf(stderr,
        "Recv bytes / time / rate: %.3f MB / %lld ms / %lld Mbps\n",
        static_cast<double>(totalRecvBytes) / 1000000.,
        durationMs,
        durationMs == 0 ? 0 : totalRecvBytes * 8 / (durationMs * 1000));

    // shutdown the connection since we're done
    iResult = shutdown(ClientSocket, SD_SEND);
    if (iResult == SOCKET_ERROR) {
        printf("shutdown failed with error: %d\n", WSAGetLastError());
        closesocket(ClientSocket);
        WSACleanup();
        return 1;
    }

    // cleanup
    closesocket(ClientSocket);
    WSACleanup();

    return 0;
}

void stringToLower(std::string &str) {
    std::transform(str.begin(), str.end(), str.begin(), ::tolower);
}

int __cdecl main(int argc, char **argv) {
    std::string socket("");
    std::string ipAddr("localhost");
    std::string ipPort("");
    int32_t mbLimit(0);
    std::string tcpFastPath("");

    switch (argc) {
    case 5:
        tcpFastPath = std::string(argv[4]);
    case 4:
        socket = std::string(argv[1]);
        ipPort = std::string(argv[2]);
        mbLimit = std::stol(argv[3]);
        break;
    default:
        break;
    }

    stringToLower(socket);
    stringToLower(tcpFastPath);

    if ((socket == "client" || socket == "server") &&
        (mbLimit > 0) &&
        (tcpFastPath.empty() || tcpFastPath == "tfp")) {
        if (socket == "client") {
            clientSend(ipAddr, ipPort, mbLimit, !tcpFastPath.empty());
        } else {
            serverRecv(ipAddr, ipPort, mbLimit, !tcpFastPath.empty());
        }
    } else {
        fprintf(stderr,
            "usage: %s [socket | client/server] [server port] [send/recv limit in MB] [enable TCP Fast Path | tfp]\n",
            argv[0]);
    }

    return 0;
}
