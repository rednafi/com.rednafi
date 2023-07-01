---
title: Implementing a simple traceroute clone in Python
date: 2023-06-01
slug: implement_traceroute_in_python
tags:
    - Python
    - Networking
    - Shell
---

I was watching this amazing lightning [talk][storytelling-with-traceroute] by [Karla
Burnett][karla-burnett] and wanted to understand how `traceroute` works in UNIX.
Traceroute is a tool that shows the route of a network packet from your computer to
another computer on the internet. It also tells you how long it takes for the packet to
reach each stop along the way.

It's useful when you want to know more about how your computer connects to other
computers on the internet. For example, if you want to visit a website, your computer
sends a request to the website’s server, which is another computer that hosts the
website. But the request doesn’t go directly from your computer to the server. It has to
pass through several other devices, such as routers, that help direct the traffic on the
internet. These devices are called hops. Traceroute shows you the list of hops that your
request goes through, and how long it takes for each hop to respond. This can help you
troubleshoot network problems, such as slow connections or unreachable websites.

This is how you usually use `traceroute`:

```sh
traceroute example.com
```

This returns:

```txt
traceroute to example.com (93.184.216.34), 64 hops max, 52 byte packets
 1  192.168.1.1 (192.168.1.1)  2.386 ms  1.976 ms  1.703 ms
 2  142-254-158-201.inf.spectrum.com (142.254.158.201)  9.970 ms  9.463 ms  9.867 ms
 3  lag-63.uparohgd02h.netops.charter.com (65.25.145.149)  52.340 ms  26.224 ms  18.094 ms
 4  lag-31.clmcohib01r.netops.charter.com (24.33.161.216)  24.277 ms  10.391 ms  16.529 ms
 5  lag-27.rcr01clevohek.netops.charter.com (65.29.1.38)  16.485 ms  16.258 ms  16.999 ms
 6  lag-416.vinnva0510w-bcr00.netops.charter.com (66.109.6.164)  23.478 ms  24.685 ms
    lag-415.vinnva0510w-bcr00.netops.charter.com (66.109.6.12)  25.211 ms
 7  lag-11.asbnva1611w-bcr00.netops.charter.com (66.109.6.30)  24.541 ms
    lag-21.asbnva1611w-bcr00.netops.charter.com (66.109.3.24)  24.574 ms
    lag-31.asbnva1611w-bcr00.netops.charter.com (107.14.18.82)  24.253 ms
 8  xe-7-3-1.cr0.chi10.tbone.rr.com (209.18.36.1)  24.283 ms  26.479 ms  45.171 ms
 9  ae-65.core1.dcb.edgecastcdn.net (152.195.64.129)  24.550 ms  24.753 ms  25.007 ms
10  93.184.216.34 (93.184.216.34)  23.998 ms  24.086 ms  24.180 ms
11  93.184.216.34 (93.184.216.34)  23.627 ms  24.238 ms  24.271 ms
```

This traceroute output draws the path of a network packet from my computer to
`example.com`'s server, which has an IP address of `93.184.216.34`. It shows that the
packet goes through 11 hops before reaching the destination. The first hop is my
router (`192.168.1.1`), the second hop is my ISP's router (`142.254.158.201`), and so
on. The last column shows the time it takes for each hop to respond in milliseconds
(ms). The lower the time, the faster the connection.

Some hops have multiple lines with different names or IP addresses. This means that
there are multiple routers at that hop that can handle the traffic, and `traceroute`
randomly picks one of them for each packet. For example, hop 7 has three routers with
names starting with `lag-11`, `lag-21`, and `lag-31`. These are probably load-balancing
routers that distribute the traffic among them.

The last hop (`93.184.216.34`) appears twice in the output. This is because traceroute
sends three packets to each hop by default, and sometimes the last hop responds to all
three packets instead of discarding them. This is not a problem and does not affect the
accuracy of the traceroute.

This is all good and dandy but I wanted to understand how `traceroute` can find out what
route a packet takes and how long it takes between each hop. So I started reading blogs
like [this][how-traceroute-works] one that does an awesome job at explaining what's
going on behind the scene. The gist of it goes as follows.

## How traceroute works

Traceroute works by sending a series of ICMP (Internet Control Message Protocol) echo
request packets, which are also known as pings, to the target IP address or URL that
you want to reach. Each packet has an associated time-to-live (TTL) value, which is a
number that indicates how many hops (or intermediate devices) the packet can pass
through before it expires and is discarded by a router. Yeah, strangely, TTL doesn't
denote any time duration here.

Traceroute starts by sending a packet with a low TTL value, usually 1. This means that
the packet can only make one hop before it expires. When a router receives this packet,
it decreases its TTL value by 1 and checks if it is 0. If it is 0, the router discards
the packet and sends back an **ICMP time exceeded message** to the source of the packet.
This message contains the IP address of the router that discarded the packet. This is
how the sender knows the IP address of the first hop (router, computer, or whatsoever).

Traceroute records the IP address and round-trip time (RTT) of each ICMP time exceeded
message it receives. The RTT is the time it takes for a packet to travel from the
source to the destination and back. It reflects the latency (or delay) between each hop.

Traceroute then increases the TTL value by 1 and sends another packet. This packet can
make 2 hops before it expires. The process repeats until traceroute reaches the
destination or a maximum TTL value, usually 30. When the returned IP is the same as the
initial destination IP, `traceroute` knows that the packet has completed the whole
journey. By doing this, traceroute can trace the route that your packets take to
reach the target IP address or URL and measure the latency between each hop. The tool
prints out the associated IPs and latencies as it jumps through different hops.

I snagged this photo from an SFU (Simon Fraser University) [slide][traceroute-slide]
that I think explains the machinery of `traceroute` quite well:

![traceroute-workflow]


## Writing a crappier version of traceroute in Python

After getting a rough idea of what's going on underneath, I wanted to write a simpler
and crappier version of `traceroute` in Python. This version would roughly perform the
following steps:

1. Establish a UDP socket connection that'd be used to send empty packets to the hops.
2. Create an ICMP socket that'd receive *ICMP time exceeded* messages.
3. Start a loop and use the UDP socket to send an empty byte with a TTL of 1 to the
first hop.
4. The TTL value of the packet would be decremented by 1 at the first hop. Once the TTL
reaches 0, the packet would be discarded, and an ICMP time exceeded message would be
returned to the sender through the ICMP socket. The sender would also receive the
address of the first hop.
5. Calculate the time delta between sending a packet and receiving the ICMP time
exceeded message. Also, capture the address of the first hop and log the time delta and
address to the console.
6. In the subsequent iterations, the TTL value will be incremented by 1 (2, 3, 4, ...)
and the steps from 1 through 5 will be repeated until it reaches the `max_hops` value,
which is set at 64.

Here's the complete self-contained implementation. I tested it on Python 3.11:

```python
# script.py
from __future__ import annotations

import socket
import sys
import time
from collections.abc import Generator
from contextlib import ExitStack


def traceroute(
    dest_addr: str, max_hops: int = 64, timeout: float = 2
) -> Generator[tuple[str, float], None, None]:
    """Traceroute implementation using UDP packets.

    Args:
        dest_addr (str): The destination address.
        max_hops (int, optional): The maximum number of hops.
        Defaults to 64.
        timeout (float, optional): The timeout for receiving packets.
        Defaults to 2.

    Yields:
        Generator[tuple[str, float], None, None]: A generator that
        yields the current address and elapsed time for each hop.

    """
    # ExitStack allows us to avoid multiple nested contextmanagers
    with ExitStack() as stack:
        # Create an ICMP socket connection for receiving packets
        rx = stack.enter_context(
            socket.socket(socket.AF_INET, socket.SOCK_RAW, socket.IPPROTO_ICMP)
        )

        # Create a UDP socket connection for sending packets
        tx = stack.enter_context(
            socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_UDP)
        )

        # Set the timeout for receiving packets
        rx.settimeout(timeout)

        # Bind the receiver socket to any available port
        rx.bind(("", 0))

        # Iterate over the TTL values
        for ttl in range(1, max_hops + 1):
            # Set the TTL value in the sender socket
            tx.setsockopt(socket.IPPROTO_IP, socket.IP_TTL, ttl)

            # Send an empty UDP packet to the destination address
            tx.sendto(b"", (dest_addr, 33434))

            try:
                # Start the timer
                start_time = time.perf_counter_ns()

                # Receive the response packet and extract the source address
                _, curr_addr = rx.recvfrom(512)
                curr_addr = curr_addr[0]

                # Stop the timer and calculate the elapsed time
                end_time = time.perf_counter_ns()
                elapsed_time = (end_time - start_time) / 1e6
            except socket.error:
                # If an error occurs while receiving the packet, set the
                # address and elapsed time as None
                curr_addr = None
                elapsed_time = None

            # Yield the current address and elapsed time
            yield curr_addr, elapsed_time

            # Break the loop if the destination address is reached
            if curr_addr == dest_addr:
                break


def main() -> None:
    # Get the destination address from command-line argument
    dest_name = sys.argv[1]
    dest_addr = socket.gethostbyname(dest_name)

    # Print the traceroute header
    print(f"Traceroute to {dest_name} ({dest_addr})")
    print(f"{'Hop':<5s}{'IP Address':<20s}{'Hostname':<50s}{'Time (ms)':<10s}")
    print("-" * 90)

    # Iterate over the traceroute results and print each hop information
    for i, (addr, elapsed_time) in enumerate(traceroute(dest_addr)):
        if addr is not None:
            try:
                # Get the hostname corresponding to the IP address
                host = socket.gethostbyaddr(addr)[0]
            except socket.error:
                host = ""
            # Print the hop information
            print(f"{i+1:<5d}{addr:<20s}{host:<50s}{elapsed_time:<10.3f} ms")
        else:
            # Print "*" for hops with no response
            print(f"{i+1:<5d}{'*':<20s}{'*':<50s}{'*':<10s}")


if __name__ == "__main__":
    main()
```

Running the script will give you the following nicely formatted output:

```sh
sudo python script.py example.com
```

```txt
Traceroute to example.com (93.184.216.34)
Hop  IP Address          Hostname                                          Time (ms)
----------------------------------------------------------------------------------------
1    192.168.1.1                                                           1.420      ms
2    142.254.158.201     142-254-158-201.inf.spectrum.com                  9.669      ms
3    65.25.145.149       lag-63.uparohgd02h.netops.charter.com             139.603    ms
4    24.33.161.216       lag-31.clmcohib01r.netops.charter.com             14.493     ms
5    65.29.1.38          lag-27.rcr01clevohek.netops.charter.com           19.221     ms
6    66.109.6.70         lag-17.vinnva0510w-bcr00.netops.charter.com       25.803     ms
7    66.109.3.24         lag-21.asbnva1611w-bcr00.netops.charter.com       24.969     ms
8    209.18.36.1         xe-7-3-1.cr0.chi10.tbone.rr.com                   24.351     ms
9    152.195.64.129      ae-65.core1.dcb.edgecastcdn.net                   25.114     ms
10   93.184.216.34                                                         23.546     ms
```

## References

* [Storytelling with traceroute][storytelling-with-traceroute]
* [How traceroute works][how-traceroute-works]
* [Traceroute machinery slide][traceroute-slide]

[storytelling-with-traceroute]: https://www.youtube.com/watch?v=xW_ALxfop7Y
[karla-burnett]: https://twitter.com/tetrakazi
[how-traceroute-works]: https://www.slashroot.in/how-does-traceroute-work-and-examples-using-traceroute-command
[traceroute-workflow]: https://github.com/rednafi/rednafi.com/assets/30027932/6aaca23e-5b54-4b83-aafa-3b3f39bee82b
[traceroute-slide]: http://www.sfu.ca/~ljilja/cnl/presentations/arman/nafips2001/sld006.htm
