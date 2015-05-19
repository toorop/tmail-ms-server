package main

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
)

// ipHaveNoReverse return true if IP heave no reverse
func ipHaveNoReverse(ip string) (bool, error) {
	if ip == "127.0.0.1" {
		return false, nil
	}
	_, err := net.LookupAddr(ip)
	if err != nil {
		if strings.HasSuffix(err.Error(), "no such host") {
			return true, nil
		}
	}
	return false, err
}

// isBlacklistedOn checks if ip is blacklisted on rblEndpoint
func isBlacklistedOn(ip, rblEndpoint string) (bool, error) {
	// reverse IP
	t := strings.Split(ip, ".")
	ip = ""
	for _, e := range t {
		ip = e + "." + ip
	}
	ip = ip + rblEndpoint
	_, err := net.LookupIP(ip)
	if err != nil {
		if strings.HasSuffix(err.Error(), "no such host") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// inGreyRbl checks if an IP is greylisted
func inGreyRbl(ip string) (bool, error) {
	mc := memcache.New("127.0.0.1:11211")
	i, err := mc.Get(ip)
	if err != nil {
		if err == memcache.ErrCacheMiss {
			err = mc.Add(&memcache.Item{
				Key:        ip,
				Value:      []byte(strconv.FormatInt(time.Now().UnixNano(), 10)),
				Expiration: 36 * 3600,
			})
			return true, err
		}
		return false, err
	}

	// ip in memcached
	addedAt, err := strconv.ParseInt(string(i.Value), 10, 64)
	if err != nil {
		return false, err
	}
	if addedAt+3600*3 < time.Now().UnixNano() {
		return true, nil
	}
	return false, nil
}
