'use client'

import { useState } from 'react'
import { Search, ChevronDown, ChevronRight, Hash, Clock, Box, Play, AlertCircle, CheckCircle2 } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'

// Sample data embedded for the showcase
const SAMPLE_TRACE = {
    "transaction_hash": "sample-tx-hash-12345",
    "start_time": "2026-01-28T18:10:48.227Z",
    "states": [
        {
            "step": 0,
            "timestamp": "2026-01-28T18:10:48.227Z",
            "operation": "contract_init",
            "contract_id": "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
            "function": "initialize",
            "arguments": ["admin", 1000000],
            "host_state": { "admin": "GDQP...4W37", "balance": 0 }
        },
        {
            "step": 1,
            "timestamp": "2026-01-28T18:10:48.227Z",
            "operation": "contract_call",
            "contract_id": "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
            "function": "mint",
            "arguments": ["GDQP...4W37", 500000],
            "return_value": true,
            "host_state": { "balance": 500000, "total_supply": 500000 },
            "memory": { "recipient": "GDQP...4W37", "temp_amount": 500000 }
        },
        {
            "step": 2,
            "timestamp": "2026-01-28T18:10:48.227Z",
            "operation": "contract_call",
            "contract_id": "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
            "function": "transfer",
            "arguments": ["GDQP...4W37", "GC3C...UMML", 100000],
            "host_state": { "balance": 400000, "total_supply": 500000 },
            "memory": { "amount": 100000, "from_balance": 500000, "to_balance": 100000 }
        },
        {
            "step": 3,
            "timestamp": "2026-01-28T18:10:48.227Z",
            "operation": "balance_check",
            "contract_id": "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
            "function": "get_balance",
            "arguments": ["GDQP...4W37"],
            "return_value": 400000,
            "memory": { "query_account": "GDQP...4W37", "result": 400000 }
        },
        {
            "step": 4,
            "timestamp": "2026-01-28T18:10:48.227Z",
            "operation": "contract_call",
            "contract_id": "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2",
            "function": "transfer",
            "arguments": ["GDQP...4W37", "GC3C...UMML", 500000],
            "error": "insufficient balance: attempted 500000, available 400000",
            "host_state": { "balance": 400000, "error_code": "INSUFFICIENT_BALANCE" },
            "memory": { "attempted_amount": 500000, "available_balance": 400000, "error_triggered": true }
        }
    ]
}

export default function ShowcasePage() {
    const [hash, setHash] = useState('')
    const [trace, setTrace] = useState<any>(null)
    const [loading, setLoading] = useState(false)
    const [expandedSteps, setExpandedSteps] = useState<number[]>([])

    const handleSearch = (e: React.FormEvent) => {
        e.preventDefault()
        if (!hash) return

        setLoading(true)
        setTrace(null)
        setExpandedSteps([])

        // Simulate API fetch
        setTimeout(() => {
            if (hash === 'sample-tx-hash-12345') {
                setTrace(SAMPLE_TRACE)
            } else {
                // Generate a random trace for any other hash to make it "interactive"
                setTrace({
                    ...SAMPLE_TRACE,
                    transaction_hash: hash,
                    states: SAMPLE_TRACE.states.slice(0, 3)
                })
            }
            setLoading(false)
        }, 800)
    }

    const toggleStep = (stepIdx: number) => {
        if (expandedSteps.includes(stepIdx)) {
            setExpandedSteps(expandedSteps.filter(i => i !== stepIdx))
        } else {
            setExpandedSteps([...expandedSteps, stepIdx])
        }
    }

    return (
        <main className="min-h-screen p-8 md:p-24">
            {/* Header */}
            <div className="max-w-4xl mx-auto mb-16 text-center">
                <motion.h1
                    className="text-5xl font-bold mb-4 gradient-text"
                    initial={{ opacity: 0, y: -20 }}
                    animate={{ opacity: 1, y: 0 }}
                >
                    ERST Trace Explorer
                </motion.h1>
                <motion.p
                    className="text-xl text-slate-400 mb-12"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ delay: 0.2 }}
                >
                    Inspect transaction execution traces with pin-point accuracy.
                </motion.p>

                {/* Search Bar */}
                <form onSubmit={handleSearch} className="relative max-w-2xl mx-auto">
                    <input
                        type="text"
                        value={hash}
                        onChange={(e) => setHash(e.target.value)}
                        placeholder="Enter Transaction Hash (Try: sample-tx-hash-12345)"
                        className="w-full glass pl-12 pr-32 py-4 text-lg"
                    />
                    <Search className="absolute left-4 top-1/2 -translate-y-1/2 text-slate-500" size={20} />
                    <button
                        type="submit"
                        className="absolute right-2 top-2 bottom-2 btn-primary px-6 py-2"
                    >
                        Explore
                    </button>
                </form>
            </div>

            {/* Results Area */}
            <div className="max-w-4xl mx-auto">
                {loading && (
                    <div className="flex flex-col items-center justify-center py-20">
                        <div className="w-12 h-12 border-4 border-accent border-t-transparent rounded-full animate-spin mb-4" />
                        <p className="text-slate-400">Navigating ledger state...</p>
                    </div>
                )}

                {trace && !loading && (
                    <motion.div
                        initial={{ opacity: 0, scale: 0.95 }}
                        animate={{ opacity: 1, scale: 1 }}
                        className="space-y-6"
                    >
                        {/* Trace Meta */}
                        <div className="glass p-6 rounded-xl flex flex-wrap gap-8 items-center">
                            <div>
                                <span className="text-xs font-bold text-accent uppercase tracking-wider block mb-1">Transaction Hash</span>
                                <div className="flex items-center gap-2">
                                    <Hash size={16} className="text-slate-500" />
                                    <span className="font-mono text-lg">{trace.transaction_hash}</span>
                                </div>
                            </div>
                            <div>
                                <span className="text-xs font-bold text-accent uppercase tracking-wider block mb-1">Execution Time</span>
                                <div className="flex items-center gap-2">
                                    <Clock size={16} className="text-slate-500" />
                                    <span>{new Date(trace.start_time).toLocaleString()}</span>
                                </div>
                            </div>
                            <div className="ml-auto">
                                <span className={`px-3 py-1 rounded-full text-xs font-bold uppercase tracking-widest ${trace.states.some((s: any) => s.error) ? 'bg-red-500/10 text-red-500 border border-red-500/20' : 'bg-green-500/10 text-green-500 border border-green-500/20'}`}>
                                    {trace.states.some((s: any) => s.error) ? 'Failed' : 'Executed'}
                                </span>
                            </div>
                        </div>

                        {/* Steps List */}
                        <div className="space-y-4">
                            <h2 className="text-xl font-semibold flex items-center gap-2">
                                <Play size={20} className="text-accent" />
                                Execution Steps ({trace.states.length})
                            </h2>

                            {trace.states.map((step: any, idx: number) => (
                                <div key={idx} className="glass rounded-xl overflow-hidden trace-item">
                                    <button
                                        onClick={() => toggleStep(idx)}
                                        className="w-full text-left p-4 flex items-center gap-4 hover:bg-white/5 transition-colors"
                                    >
                                        <div className={`w-8 h-8 rounded-lg flex items-center justify-center text-sm font-bold ${step.error ? 'bg-red-500/20 text-red-500' : 'bg-accent/20 text-accent'}`}>
                                            {step.step}
                                        </div>
                                        <div className="flex-1">
                                            <div className="flex items-center gap-2">
                                                <span className="font-mono text-sm opacity-60 uppercase">{step.operation}</span>
                                                <span className="text-slate-500">|</span>
                                                <span className="font-bold">{step.function || 'n/a'}</span>
                                            </div>
                                            <div className="text-xs text-slate-500 font-mono truncate max-w-md">
                                                {step.contract_id}
                                            </div>
                                        </div>
                                        {step.error && (
                                            <div className="flex items-center gap-1 text-red-400 text-sm">
                                                <AlertCircle size={14} />
                                                <span>Error</span>
                                            </div>
                                        )}
                                        {expandedSteps.includes(idx) ? <ChevronDown size={20} className="text-slate-500" /> : <ChevronRight size={20} className="text-slate-500" />}
                                    </button>

                                    <AnimatePresence>
                                        {expandedSteps.includes(idx) && (
                                            <motion.div
                                                initial={{ height: 0, opacity: 0 }}
                                                animate={{ height: 'auto', opacity: 1 }}
                                                exit={{ height: 0, opacity: 0 }}
                                                className="border-t border-border bg-black/40"
                                            >
                                                <div className="p-4 grid md:grid-cols-2 gap-4">
                                                    <section>
                                                        <h4 className="text-xs font-bold text-slate-500 uppercase mb-2">Host State Change</h4>
                                                        <pre>{JSON.stringify(step.host_state, null, 2)}</pre>
                                                    </section>
                                                    <section>
                                                        <h4 className="text-xs font-bold text-slate-500 uppercase mb-2">Memory View</h4>
                                                        <pre>{JSON.stringify(step.memory || {}, null, 2)}</pre>
                                                    </section>
                                                    {step.arguments && (
                                                        <section className="md:col-span-2">
                                                            <h4 className="text-xs font-bold text-slate-500 uppercase mb-2">Arguments</h4>
                                                            <div className="flex gap-2">
                                                                {step.arguments.map((arg: any, i: number) => (
                                                                    <span key={i} className="px-2 py-1 bg-white/5 rounded font-mono text-xs">
                                                                        {typeof arg === 'object' ? JSON.stringify(arg) : String(arg)}
                                                                    </span>
                                                                ))}
                                                            </div>
                                                        </section>
                                                    )}
                                                    {step.error && (
                                                        <section className="md:col-span-2 p-3 bg-red-500/10 rounded border border-red-500/20">
                                                            <h4 className="text-xs font-bold text-red-500 uppercase mb-1">Stack Trace / Error Message</h4>
                                                            <p className="text-sm font-mono text-red-400">{step.error}</p>
                                                        </section>
                                                    )}
                                                </div>
                                            </motion.div>
                                        )}
                                    </AnimatePresence>
                                </div>
                            ))}
                        </div>
                    </motion.div>
                )}

                {!trace && !loading && hash && (
                    <div className="text-center py-20 bg-white/5 rounded-2xl border border-dashed border-border">
                        <AlertCircle className="mx-auto text-slate-500 mb-4" size={48} />
                        <h3 className="text-xl font-bold mb-2">Trace not found</h3>
                        <p className="text-slate-500">The hash {hash} doesn't exist in our records yet.</p>
                    </div>
                )}
            </div>

            {/* Background Decorative Elements */}
            <div className="fixed top-0 left-0 w-full h-full pointer-events-none -z-10 overflow-hidden">
                <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-accent/20 rounded-full blur-[120px]" />
                <div className="absolute bottom-[-10%] right-[-10%] w-[30%] h-[30%] bg-purple-500/10 rounded-full blur-[100px]" />
            </div>
        </main>
    )
}
