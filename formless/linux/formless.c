#include <linux/inet.h>
#include <linux/ip.h>
#include <linux/kernel.h>
#include <linux/module.h>
#include <linux/moduleparam.h>
#include <linux/netfilter.h>
#include <linux/netfilter_ipv4.h>
#include <linux/skbuff.h>
#include <linux/string.h>
#include <linux/udp.h>

MODULE_AUTHOR("Shane Barnes");
MODULE_DESCRIPTION("Create custom network traffic characteristics");
MODULE_LICENSE("GPL");
MODULE_VERSION("0.0.1") ;

static char *module = "formless";

static struct nf_hook_ops nf_ops_pre_route;
static struct nf_hook_ops nf_ops_post_route;

static char *form[16];
static int form_count=0;

struct form_rule {
    int ip_proto;
};
static struct form_rule *rules = NULL;

module_param_array(form, charp, &form_count, 0000);
MODULE_PARM_DESC(form, "form arguments (e.g., form='tcp','udp')");

unsigned int hook_callback(unsigned int hook, struct sk_buff *skb, const struct net_device *in, const struct net_device *out, int (*okfn)(struct sk_buff*))
{
        unsigned int verdict = NF_ACCEPT;
        printk(KERN_DEBUG "%s.%s: hook %d called\n", module, __FUNCTION__, hook);

        if(skb && in && in->flags&IFF_LOOPBACK) {
                //printk(KERN_INFO "%s: processing socket buffer\n", module);
                struct iphdr *ip4 = (struct iphdr*)skb_network_header(skb);
                if (ip4->protocol == IPPROTO_UDP) {
                        //            char src[INET_ADDRSTRLEN];
                        //            inet_ntop(AF_INET, &ip_header->saddr, src, INET_ADDRSTRLEN);
                        struct udphdr *udp = (struct udphdr*)skb_transport_header(skb);
                        if (ntohs(udp->source) == 49221 && ntohs(udp->len) > 1400) {
                                printk(KERN_INFO "%s.%s: Dropping udp packet %pI4:%d -> %pI4:%d, len: %d\n",
                                        module,
                                        __FUNCTION__,
                                        &ip4->saddr,
                                        ntohs(udp->source),
                                        &ip4->daddr,
                                        ntohs(udp->dest),
                                        ntohs(udp->len));
                                verdict = NF_DROP;
                        }
                }
        }

        return verdict;
}

int parse_ipproto(char *str)
{
        int proto = -1;

        if (strncasecmp("icmp", str, 4) == 0) {
                proto = IPPROTO_ICMP;
        } else if (strncasecmp("tcp", str, 3) == 0) {
                proto = IPPROTO_TCP;
        } else if (strncasecmp("udp", str, 3) == 0) {
                proto = IPPROTO_UDP;
        }

        return proto;
}

int init_module()
{
        int i, result = -1;

        if (form_count) {
                rules = kmalloc(sizeof(struct form_rule)*form_count, GFP_KERNEL);
                if (rules == NULL) {
                        printk(KERN_ERR "%s.%s: failed to allocate memory\n", module, __FUNCTION__);
                        goto init_module_return;
                } else {
                        for (i = 0; i < form_count; i++) {
                                printk(KERN_INFO "%s.%s: form='%s'\n", module, __FUNCTION__, form[i]);
                                rules[i].ip_proto = parse_ipproto(form[i]);
                                if (rules[i].ip_proto < 0) {
                                        printk(KERN_ERR "%s.%s: invalid protocol: %s\n", module, __FUNCTION__, form[i]);
                                        goto init_module_return;
                                }
                        }
                }

                result = 0;

                printk(KERN_INFO "%s.%s: registering pre-routing hook\n", module, __FUNCTION__);
                nf_ops_pre_route.hook = (nf_hookfn*)hook_callback;
                nf_ops_pre_route.hooknum = NF_INET_PRE_ROUTING;
                nf_ops_pre_route.pf = PF_INET;
                nf_ops_pre_route.priority = NF_IP_PRI_FIRST;
                result = nf_register_hook(&nf_ops_pre_route);
                if (result != 0) {
                        printk(KERN_ERR "%s.%s: failed to register pre-routing hook: %d\n", module, __FUNCTION__, result);
                        goto init_module_return;
                }

                printk(KERN_INFO "%s.%s: registering post-routing hook\n", module, __FUNCTION__);
                nf_ops_post_route.hook = (nf_hookfn*)hook_callback;
                nf_ops_post_route.hooknum = NF_INET_POST_ROUTING;
                nf_ops_post_route.pf = PF_INET;
                nf_ops_post_route.priority = NF_IP_PRI_FIRST;
                result = nf_register_hook(&nf_ops_post_route);
                if (result != 0) {
                        printk(KERN_ERR "%s.%s: failed to register post-routing hook: %d\n", module, __FUNCTION__, result);
                        goto init_module_return;
                }

                printk(KERN_INFO "%s.%s: module loaded\n", module, __FUNCTION__);
        } else {
                printk(KERN_ERR "%s.%s: no module parameters were provided\n", module, __FUNCTION__);
        }

init_module_return:
        if (result && rules != NULL) {
                kfree(rules);
                rules = NULL;
        }
        return result;
}

void cleanup_module()
{
        printk(KERN_INFO "%s.%s: unregistering pre-routing hook\n", module, __FUNCTION__);
        nf_unregister_hook(&nf_ops_pre_route);

        printk(KERN_INFO "%s.%s: unregistering post-routing hook\n", module, __FUNCTION__);
        nf_unregister_hook(&nf_ops_post_route);

        if (rules) {
                kfree(rules);
                rules = NULL;
        }

        printk(KERN_INFO "%s.%s: module unloaded\n", module, __FUNCTION__);
}
