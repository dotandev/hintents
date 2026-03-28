// Copyright (c) 2026 dotandev
// SPDX-License-Identifier: MIT OR Apache-2.0

'use client'

import { useState, useMemo, useCallback } from 'react'
import {
    GitCompare, Download, MessageSquare, X, ArrowLeft,
    Plus, Minus, FileCode, ChevronRight,
} from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'
import Link from 'next/link'

// ===== Sample Contract Versions =====

const CONTRACT_VERSIONS: Record<string, string> = {
    'v1.0.0': `use soroban_sdk::{contract, contractimpl, Address, Env, Symbol};

#[contract]
pub struct TokenContract;

#[contractimpl]
impl TokenContract {
    pub fn initialize(env: Env, admin: Address, decimals: u32) {
        env.storage().instance().set(&Symbol::new(&env, "admin"), &admin);
        env.storage().instance().set(&Symbol::new(&env, "decimals"), &decimals);
        env.storage().instance().set(&Symbol::new(&env, "total_supply"), &0u64);
    }

    pub fn mint(env: Env, to: Address, amount: u64) {
        let admin: Address = env.storage().instance()
            .get(&Symbol::new(&env, "admin")).unwrap();
        admin.require_auth();

        let balance: u64 = Self::balance(env.clone(), to.clone());
        env.storage().persistent().set(&to, &(balance + amount));

        let supply: u64 = env.storage().instance()
            .get(&Symbol::new(&env, "total_supply")).unwrap_or(0);
        env.storage().instance()
            .set(&Symbol::new(&env, "total_supply"), &(supply + amount));
    }

    pub fn transfer(env: Env, from: Address, to: Address, amount: u64) {
        from.require_auth();
        let from_balance = Self::balance(env.clone(), from.clone());
        assert!(from_balance >= amount, "insufficient balance");
        env.storage().persistent().set(&from, &(from_balance - amount));
        let to_balance = Self::balance(env.clone(), to.clone());
        env.storage().persistent().set(&to, &(to_balance + amount));
    }

    pub fn balance(env: Env, account: Address) -> u64 {
        env.storage().persistent().get(&account).unwrap_or(0)
    }
}`,

    'v1.1.0': `use soroban_sdk::{contract, contractimpl, Address, Env, Symbol};

const FEE_BPS: u64 = 30;

#[contract]
pub struct TokenContract;

#[contractimpl]
impl TokenContract {
    pub fn initialize(env: Env, admin: Address, decimals: u32) {
        env.storage().instance().set(&Symbol::new(&env, "admin"), &admin);
        env.storage().instance().set(&Symbol::new(&env, "decimals"), &decimals);
        env.storage().instance().set(&Symbol::new(&env, "total_supply"), &0u64);
        env.storage().instance().set(&Symbol::new(&env, "fee_enabled"), &false);
    }

    pub fn mint(env: Env, to: Address, amount: u64) {
        let admin: Address = env.storage().instance()
            .get(&Symbol::new(&env, "admin")).unwrap();
        admin.require_auth();
        assert!(amount > 0, "amount must be positive");

        let balance: u64 = Self::balance(env.clone(), to.clone());
        let new_balance = balance.checked_add(amount).expect("balance overflow");
        env.storage().persistent().set(&to, &new_balance);

        let supply: u64 = env.storage().instance()
            .get(&Symbol::new(&env, "total_supply")).unwrap_or(0);
        let new_supply = supply.checked_add(amount).expect("supply overflow");
        env.storage().instance()
            .set(&Symbol::new(&env, "total_supply"), &new_supply);

        env.events().publish((Symbol::new(&env, "mint"),), (to, amount));
    }

    pub fn transfer(env: Env, from: Address, to: Address, amount: u64) {
        from.require_auth();
        assert!(amount > 0, "amount must be positive");
        let from_balance = Self::balance(env.clone(), from.clone());
        assert!(from_balance >= amount, "insufficient balance");

        let fee_enabled: bool = env.storage().instance()
            .get(&Symbol::new(&env, "fee_enabled")).unwrap_or(false);
        let fee = if fee_enabled { amount * FEE_BPS / 10_000 } else { 0 };
        let net_amount = amount - fee;

        env.storage().persistent().set(&from, &(from_balance - amount));
        let to_balance = Self::balance(env.clone(), to.clone());
        let new_to = to_balance.checked_add(net_amount).expect("overflow");
        env.storage().persistent().set(&to, &new_to);

        env.events().publish((Symbol::new(&env, "transfer"),), (from, to, net_amount, fee));
    }

    pub fn set_fee(env: Env, enabled: bool) {
        let admin: Address = env.storage().instance()
            .get(&Symbol::new(&env, "admin")).unwrap();
        admin.require_auth();
        env.storage().instance().set(&Symbol::new(&env, "fee_enabled"), &enabled);
    }

    pub fn balance(env: Env, account: Address) -> u64 {
        env.storage().persistent().get(&account).unwrap_or(0)
    }
}`,

    'v1.2.0': `use soroban_sdk::{contract, contractimpl, Address, Env, Symbol, log};

const FEE_BPS: u64 = 30;
const MAX_SUPPLY: u64 = 1_000_000_000_000;

#[contract]
pub struct TokenContract;

#[contractimpl]
impl TokenContract {
    pub fn initialize(env: Env, admin: Address, decimals: u32, name: Symbol) {
        env.storage().instance().set(&Symbol::new(&env, "admin"), &admin);
        env.storage().instance().set(&Symbol::new(&env, "decimals"), &decimals);
        env.storage().instance().set(&Symbol::new(&env, "total_supply"), &0u64);
        env.storage().instance().set(&Symbol::new(&env, "fee_enabled"), &false);
        env.storage().instance().set(&Symbol::new(&env, "name"), &name);
        env.storage().instance().set(&Symbol::new(&env, "paused"), &false);
    }

    pub fn mint(env: Env, to: Address, amount: u64) {
        Self::require_not_paused(&env);
        let admin: Address = env.storage().instance()
            .get(&Symbol::new(&env, "admin")).unwrap();
        admin.require_auth();
        assert!(amount > 0, "amount must be positive");

        let supply: u64 = env.storage().instance()
            .get(&Symbol::new(&env, "total_supply")).unwrap_or(0);
        assert!(supply + amount <= MAX_SUPPLY, "exceeds max supply");

        let balance: u64 = Self::balance(env.clone(), to.clone());
        let new_balance = balance.checked_add(amount).expect("balance overflow");
        env.storage().persistent().set(&to, &new_balance);

        let new_supply = supply.checked_add(amount).expect("supply overflow");
        env.storage().instance()
            .set(&Symbol::new(&env, "total_supply"), &new_supply);

        log!(&env, "minted {} to {}", amount, to);
        env.events().publish((Symbol::new(&env, "mint"),), (to, amount));
    }

    pub fn transfer(env: Env, from: Address, to: Address, amount: u64) {
        Self::require_not_paused(&env);
        from.require_auth();
        assert!(amount > 0, "amount must be positive");
        let from_balance = Self::balance(env.clone(), from.clone());
        assert!(from_balance >= amount, "insufficient balance");

        let fee_enabled: bool = env.storage().instance()
            .get(&Symbol::new(&env, "fee_enabled")).unwrap_or(false);
        let fee = if fee_enabled { amount * FEE_BPS / 10_000 } else { 0 };
        let net_amount = amount - fee;

        env.storage().persistent().set(&from, &(from_balance - amount));
        let to_balance = Self::balance(env.clone(), to.clone());
        let new_to = to_balance.checked_add(net_amount).expect("overflow");
        env.storage().persistent().set(&to, &new_to);

        env.events().publish((Symbol::new(&env, "transfer"),), (from, to, net_amount, fee));
    }

    pub fn set_fee(env: Env, enabled: bool) {
        let admin: Address = env.storage().instance()
            .get(&Symbol::new(&env, "admin")).unwrap();
        admin.require_auth();
        env.storage().instance().set(&Symbol::new(&env, "fee_enabled"), &enabled);
    }

    pub fn pause(env: Env) {
        let admin: Address = env.storage().instance()
            .get(&Symbol::new(&env, "admin")).unwrap();
        admin.require_auth();
        env.storage().instance().set(&Symbol::new(&env, "paused"), &true);
    }

    pub fn unpause(env: Env) {
        let admin: Address = env.storage().instance()
            .get(&Symbol::new(&env, "admin")).unwrap();
        admin.require_auth();
        env.storage().instance().set(&Symbol::new(&env, "paused"), &false);
    }

    fn require_not_paused(env: &Env) {
        let paused: bool = env.storage().instance()
            .get(&Symbol::new(env, "paused")).unwrap_or(false);
        assert!(!paused, "contract is paused");
    }

    pub fn balance(env: Env, account: Address) -> u64 {
        env.storage().persistent().get(&account).unwrap_or(0)
    }
}`,
}

// ===== Types =====

type DiffLineType = 'add' | 'remove' | 'context'

interface DiffLine {
    type: DiffLineType
    content: string
    oldLineNum?: number
    newLineNum?: number
}

interface SideBySidePair {
    left?: DiffLine
    right?: DiffLine
}

interface LineComment {
    id: string
    lineKey: string
    text: string
    postedAt: string
}

// ===== Diff Algorithm (LCS) =====

function computeDiff(oldLines: string[], newLines: string[]): DiffLine[] {
    const m = oldLines.length
    const n = newLines.length
    const dp: number[][] = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0))

    for (let i = 1; i <= m; i++)
        for (let j = 1; j <= n; j++)
            dp[i][j] = oldLines[i - 1] === newLines[j - 1]
                ? dp[i - 1][j - 1] + 1
                : Math.max(dp[i - 1][j], dp[i][j - 1])

    const raw: { type: DiffLineType; content: string }[] = []
    let i = m, j = n

    while (i > 0 || j > 0) {
        if (i > 0 && j > 0 && oldLines[i - 1] === newLines[j - 1]) {
            raw.unshift({ type: 'context', content: oldLines[i - 1] })
            i--; j--
        } else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
            raw.unshift({ type: 'add', content: newLines[j - 1] })
            j--
        } else {
            raw.unshift({ type: 'remove', content: oldLines[i - 1] })
            i--
        }
    }

    let oldNum = 1, newNum = 1
    return raw.map(line => {
        const dl: DiffLine = { ...line }
        if (line.type === 'context') { dl.oldLineNum = oldNum++; dl.newLineNum = newNum++ }
        else if (line.type === 'remove') { dl.oldLineNum = oldNum++ }
        else { dl.newLineNum = newNum++ }
        return dl
    })
}

function toSideBySide(diff: DiffLine[]): SideBySidePair[] {
    const pairs: SideBySidePair[] = []
    let i = 0

    while (i < diff.length) {
        if (diff[i].type === 'context') {
            pairs.push({ left: diff[i], right: diff[i] })
            i++
        } else {
            const removes: DiffLine[] = []
            const adds: DiffLine[] = []
            while (i < diff.length && diff[i].type === 'remove') removes.push(diff[i++])
            while (i < diff.length && diff[i].type === 'add') adds.push(diff[i++])
            const len = Math.max(removes.length, adds.length)
            for (let k = 0; k < len; k++)
                pairs.push({ left: removes[k], right: adds[k] })
        }
    }

    return pairs
}

function generatePatch(diff: DiffLine[], oldFile: string, newFile: string): string {
    const CTX = 3
    let patch = `--- a/${oldFile}\n+++ b/${newFile}\n`

    const changeIdxs = diff.flatMap((l, idx) => l.type !== 'context' ? [idx] : [])
    if (!changeIdxs.length) return patch

    const hunks: [number, number][] = []
    let hs = Math.max(0, changeIdxs[0] - CTX)
    let he = Math.min(diff.length - 1, changeIdxs[0] + CTX)

    for (let k = 1; k < changeIdxs.length; k++) {
        if (changeIdxs[k] - CTX <= he + CTX) {
            he = Math.min(diff.length - 1, changeIdxs[k] + CTX)
        } else {
            hunks.push([hs, he])
            hs = Math.max(0, changeIdxs[k] - CTX)
            he = Math.min(diff.length - 1, changeIdxs[k] + CTX)
        }
    }
    hunks.push([hs, he])

    for (const [start, end] of hunks) {
        const lines = diff.slice(start, end + 1)
        const oldStart = lines.find(l => l.oldLineNum != null)?.oldLineNum ?? 1
        const newStart = lines.find(l => l.newLineNum != null)?.newLineNum ?? 1
        const oldCount = lines.filter(l => l.type !== 'add').length
        const newCount = lines.filter(l => l.type !== 'remove').length
        patch += `@@ -${oldStart},${oldCount} +${newStart},${newCount} @@\n`
        for (const l of lines)
            patch += `${l.type === 'add' ? '+' : l.type === 'remove' ? '-' : ' '}${l.content}\n`
    }

    return patch
}

// ===== Rust Syntax Highlighter =====

const RUST_KEYWORDS = new Set([
    'as', 'async', 'await', 'break', 'const', 'continue', 'crate', 'dyn',
    'else', 'enum', 'extern', 'false', 'fn', 'for', 'if', 'impl', 'in',
    'let', 'loop', 'match', 'mod', 'move', 'mut', 'pub', 'ref', 'return',
    'self', 'Self', 'static', 'struct', 'super', 'trait', 'true', 'type',
    'unsafe', 'use', 'where', 'while',
])

const RUST_TYPES = new Set([
    'u8', 'u16', 'u32', 'u64', 'u128', 'i8', 'i16', 'i32', 'i64', 'i128',
    'f32', 'f64', 'usize', 'isize', 'bool', 'char', 'str', 'String', 'Vec',
    'Option', 'Result', 'Box', 'Rc', 'Arc', 'HashMap', 'HashSet', 'BTreeMap',
    'Address', 'Env', 'Symbol', 'Map', 'Bytes', 'Events',
])

function tokenizeRust(line: string): { text: string; cls: string }[] {
    const tokens: { text: string; cls: string }[] = []
    let i = 0

    while (i < line.length) {
        if (line[i] === '/' && line[i + 1] === '/') {
            tokens.push({ text: line.slice(i), cls: 'syn-comment' }); break
        }
        if (line[i] === '#' && line[i + 1] === '[') {
            const end = line.indexOf(']', i)
            const stop = end >= 0 ? end + 1 : line.length
            tokens.push({ text: line.slice(i, stop), cls: 'syn-attr' }); i = stop; continue
        }
        if (line[i] === '"') {
            let j = i + 1
            while (j < line.length && (line[j] !== '"' || line[j - 1] === '\\')) j++
            tokens.push({ text: line.slice(i, j + 1), cls: 'syn-string' }); i = j + 1; continue
        }
        if (line[i] === "'" && /[a-z_]/.test(line[i + 1] ?? '')) {
            let j = i + 1
            while (j < line.length && /\w/.test(line[j])) j++
            tokens.push({ text: line.slice(i, j), cls: 'syn-lifetime' }); i = j; continue
        }
        if (/[0-9]/.test(line[i])) {
            let j = i
            while (j < line.length && /[0-9_a-zA-Z.]/.test(line[j])) j++
            tokens.push({ text: line.slice(i, j), cls: 'syn-number' }); i = j; continue
        }
        if (/[a-zA-Z_]/.test(line[i])) {
            let j = i
            while (j < line.length && /\w/.test(line[j])) j++
            const word = line.slice(i, j)
            if (line[j] === '!') {
                tokens.push({ text: word + '!', cls: 'syn-macro' }); i = j + 1
            } else if (RUST_KEYWORDS.has(word)) {
                tokens.push({ text: word, cls: 'syn-keyword' }); i = j
            } else if (RUST_TYPES.has(word)) {
                tokens.push({ text: word, cls: 'syn-type' }); i = j
            } else if (line[j] === '(' && /[a-z_]/.test(word[0])) {
                tokens.push({ text: word, cls: 'syn-fn' }); i = j
            } else {
                tokens.push({ text: word, cls: 'syn-plain' }); i = j
            }
            continue
        }
        tokens.push({ text: line[i], cls: 'syn-op' }); i++
    }

    return tokens
}

function HighlightedCode({ code }: { code: string }) {
    if (!code) return <span className="syn-plain">&nbsp;</span>
    const tokens = tokenizeRust(code)
    return <>{tokens.map((t, i) => <span key={i} className={t.cls}>{t.text}</span>)}</>
}

// ===== Comment Box =====

function CommentBox({ lineKey, all, onAdd, onClose }: {
    lineKey: string
    all: LineComment[]
    onAdd: (key: string, text: string) => void
    onClose: () => void
}) {
    const [input, setInput] = useState('')
    const mine = all.filter(c => c.lineKey === lineKey)

    const submit = () => {
        if (!input.trim()) return
        onAdd(lineKey, input.trim())
        setInput('')
    }

    return (
        <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            style={{ overflow: 'hidden' }}
        >
            <div className="diff-comment-box">
                <div className="diff-comment-header">
                    <span className="diff-comment-title">
                        <MessageSquare size={12} />
                        Comments &mdash; line {lineKey.replace(/\D+/g, '')}
                    </span>
                    <button className="diff-comment-close" onClick={onClose}>
                        <X size={13} />
                    </button>
                </div>
                {mine.map(c => (
                    <div key={c.id} className="diff-comment-entry">
                        <p className="diff-comment-text">{c.text}</p>
                        <p className="diff-comment-time">{c.postedAt}</p>
                    </div>
                ))}
                <div className="diff-comment-input-row">
                    <input
                        value={input}
                        onChange={e => setInput(e.target.value)}
                        placeholder="Write a comment…"
                        className="diff-comment-input"
                        onKeyDown={e => e.key === 'Enter' && submit()}
                    />
                    <button onClick={submit} className="btn-primary diff-comment-post">
                        Post
                    </button>
                </div>
            </div>
        </motion.div>
    )
}

// ===== Diff Line Cell (reused across views) =====

function CommentToggle({ lineKey, count, isActive, onToggle }: {
    lineKey: string
    count: number
    isActive: boolean
    onToggle: () => void
}) {
    return (
        <button
            className={`diff-comment-btn${isActive || count > 0 ? ' diff-comment-btn--active' : ''}`}
            onClick={onToggle}
            title="Comment on this line"
        >
            <MessageSquare size={11} />
            {count > 0 && <span className="diff-comment-count">{count}</span>}
        </button>
    )
}

// ===== Side-by-Side View =====

function SideBySideView({ pairs, fromVersion, toVersion, comments, activeComment, onToggle, onAdd, onClose }: {
    pairs: SideBySidePair[]
    fromVersion: string
    toVersion: string
    comments: LineComment[]
    activeComment: string | null
    onToggle: (key: string) => void
    onAdd: (key: string, text: string) => void
    onClose: () => void
}) {
    return (
        <div className="diff-sbs-container">
            {/* Column headers */}
            <div className="diff-sbs-colheads">
                <div className="diff-sbs-colhead diff-sbs-colhead--left">
                    <FileCode size={13} />
                    {fromVersion}
                </div>
                <div className="diff-sbs-colhead diff-sbs-colhead--right">
                    <FileCode size={13} />
                    {toVersion}
                </div>
            </div>

            {/* Rows */}
            {pairs.map((pair, idx) => {
                const leftKey = pair.left?.oldLineNum != null ? `old-${pair.left.oldLineNum}` : null
                const rightKey = pair.right?.newLineNum != null ? `new-${pair.right.newLineNum}` : null
                const leftCount = leftKey ? comments.filter(c => c.lineKey === leftKey).length : 0
                const rightCount = rightKey ? comments.filter(c => c.lineKey === rightKey).length : 0
                const commentKey = activeComment === leftKey ? leftKey
                    : activeComment === rightKey ? rightKey
                    : null

                const leftBg = pair.left?.type === 'remove' ? 'diff-remove'
                    : pair.left?.type === 'context' ? 'diff-context'
                    : 'diff-empty'
                const rightBg = pair.right?.type === 'add' ? 'diff-add'
                    : pair.right?.type === 'context' ? 'diff-context'
                    : 'diff-empty'

                return (
                    <div key={idx} className="diff-sbs-row-group">
                        <div className="diff-sbs-row">
                            {/* Left cell */}
                            <div className={`diff-cell diff-cell--left ${leftBg}`}>
                                <span className="diff-linenum">{pair.left?.oldLineNum ?? ''}</span>
                                <span className="diff-prefix">
                                    {pair.left?.type === 'remove' ? <Minus size={10} className="diff-icon-remove" /> : null}
                                    {pair.left?.type === 'context' ? ' ' : null}
                                </span>
                                <code className="diff-code">
                                    <HighlightedCode code={pair.left?.content ?? ''} />
                                </code>
                                {leftKey && (
                                    <CommentToggle
                                        lineKey={leftKey}
                                        count={leftCount}
                                        isActive={activeComment === leftKey}
                                        onToggle={() => onToggle(leftKey)}
                                    />
                                )}
                            </div>

                            {/* Right cell */}
                            <div className={`diff-cell diff-cell--right ${rightBg}`}>
                                <span className="diff-linenum">{pair.right?.newLineNum ?? ''}</span>
                                <span className="diff-prefix">
                                    {pair.right?.type === 'add' ? <Plus size={10} className="diff-icon-add" /> : null}
                                    {pair.right?.type === 'context' ? ' ' : null}
                                </span>
                                <code className="diff-code">
                                    <HighlightedCode code={pair.right?.content ?? ''} />
                                </code>
                                {rightKey && (
                                    <CommentToggle
                                        lineKey={rightKey}
                                        count={rightCount}
                                        isActive={activeComment === rightKey}
                                        onToggle={() => onToggle(rightKey)}
                                    />
                                )}
                            </div>
                        </div>

                        <AnimatePresence>
                            {commentKey && (
                                <div className="diff-comment-row">
                                    <CommentBox
                                        lineKey={commentKey}
                                        all={comments}
                                        onAdd={onAdd}
                                        onClose={onClose}
                                    />
                                </div>
                            )}
                        </AnimatePresence>
                    </div>
                )
            })}
        </div>
    )
}

// ===== Unified View =====

function UnifiedView({ diff, comments, activeComment, onToggle, onAdd, onClose }: {
    diff: DiffLine[]
    comments: LineComment[]
    activeComment: string | null
    onToggle: (key: string) => void
    onAdd: (key: string, text: string) => void
    onClose: () => void
}) {
    return (
        <div className="diff-unified-container">
            {diff.map((line, idx) => {
                const lineKey = line.type === 'add' ? `new-${line.newLineNum}`
                    : line.type === 'remove' ? `old-${line.oldLineNum}`
                    : `ctx-${line.oldLineNum}`
                const count = comments.filter(c => c.lineKey === lineKey).length
                const rowCls = line.type === 'add' ? 'diff-add'
                    : line.type === 'remove' ? 'diff-remove'
                    : 'diff-context'

                return (
                    <div key={idx} className="diff-unified-row-group">
                        <div className={`diff-unified-row ${rowCls}`}>
                            <span className="diff-linenum diff-linenum-old">{line.oldLineNum ?? ''}</span>
                            <span className="diff-linenum diff-linenum-new">{line.newLineNum ?? ''}</span>
                            <span className="diff-prefix">
                                {line.type === 'add' ? <Plus size={10} className="diff-icon-add" /> : null}
                                {line.type === 'remove' ? <Minus size={10} className="diff-icon-remove" /> : null}
                                {line.type === 'context' ? ' ' : null}
                            </span>
                            <code className="diff-code">
                                <HighlightedCode code={line.content} />
                            </code>
                            <CommentToggle
                                lineKey={lineKey}
                                count={count}
                                isActive={activeComment === lineKey}
                                onToggle={() => onToggle(lineKey)}
                            />
                        </div>
                        <AnimatePresence>
                            {activeComment === lineKey && (
                                <div className="diff-comment-row">
                                    <CommentBox
                                        lineKey={lineKey}
                                        all={comments}
                                        onAdd={onAdd}
                                        onClose={onClose}
                                    />
                                </div>
                            )}
                        </AnimatePresence>
                    </div>
                )
            })}
        </div>
    )
}

// ===== Main Page =====

export default function DiffViewerPage() {
    const versionKeys = Object.keys(CONTRACT_VERSIONS)
    const [fromVersion, setFromVersion] = useState(versionKeys[0])
    const [toVersion, setToVersion] = useState(versionKeys[1])
    const [viewMode, setViewMode] = useState<'side-by-side' | 'unified'>('side-by-side')
    const [comments, setComments] = useState<LineComment[]>([])
    const [activeComment, setActiveComment] = useState<string | null>(null)

    const diff = useMemo(() => {
        const oldLines = CONTRACT_VERSIONS[fromVersion].split('\n')
        const newLines = CONTRACT_VERSIONS[toVersion].split('\n')
        return computeDiff(oldLines, newLines)
    }, [fromVersion, toVersion])

    const stats = useMemo(() => ({
        added: diff.filter(l => l.type === 'add').length,
        removed: diff.filter(l => l.type === 'remove').length,
    }), [diff])

    const pairs = useMemo(
        () => viewMode === 'side-by-side' ? toSideBySide(diff) : [],
        [diff, viewMode]
    )

    const handleAddComment = useCallback((key: string, text: string) => {
        setComments(prev => [...prev, {
            id: Math.random().toString(36).slice(2, 9),
            lineKey: key,
            text,
            postedAt: new Date().toLocaleTimeString(),
        }])
    }, [])

    const handleToggleComment = useCallback((key: string) => {
        setActiveComment(prev => prev === key ? null : key)
    }, [])

    const handleDownload = () => {
        const filename = `token_contract.rs`
        const patch = generatePatch(diff, filename, filename)
        const blob = new Blob([patch], { type: 'text/plain' })
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `${fromVersion}_to_${toVersion}.patch`
        a.click()
        URL.revokeObjectURL(url)
    }

    return (
        <main className="diff-page">
            {/* Background */}
            <div className="diff-bg">
                <div className="diff-bg-blob diff-bg-blob--tl" />
                <div className="diff-bg-blob diff-bg-blob--br" />
            </div>

            <div className="diff-content">
                {/* Back link */}
                <Link href="/" className="diff-back-link">
                    <ArrowLeft size={15} />
                    Trace Explorer
                </Link>

                {/* Header */}
                <motion.div
                    initial={{ opacity: 0, y: -14 }}
                    animate={{ opacity: 1, y: 0 }}
                    className="diff-header"
                >
                    <div className="diff-header-icon">
                        <GitCompare size={22} />
                    </div>
                    <div>
                        <h1 className="diff-title gradient-text">Contract Diff Viewer</h1>
                        <p className="diff-subtitle">Inspect code changes between contract versions with syntax highlighting</p>
                    </div>
                </motion.div>

                {/* Controls bar */}
                <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ delay: 0.08 }}
                    className="glass diff-controls"
                >
                    {/* Version selectors */}
                    <div className="diff-version-row">
                        <div className="diff-version-group">
                            <label className="diff-label">From</label>
                            <select
                                value={fromVersion}
                                onChange={e => setFromVersion(e.target.value)}
                                className="diff-select"
                            >
                                {versionKeys.map(v => <option key={v}>{v}</option>)}
                            </select>
                        </div>
                        <ChevronRight size={16} className="diff-arrow" />
                        <div className="diff-version-group">
                            <label className="diff-label">To</label>
                            <select
                                value={toVersion}
                                onChange={e => setToVersion(e.target.value)}
                                className="diff-select"
                            >
                                {versionKeys.map(v => <option key={v}>{v}</option>)}
                            </select>
                        </div>
                    </div>

                    <div className="diff-actions">
                        {/* Stats */}
                        <span className="diff-stat diff-stat--add">
                            <Plus size={11} /> {stats.added} added
                        </span>
                        <span className="diff-stat diff-stat--remove">
                            <Minus size={11} /> {stats.removed} removed
                        </span>

                        {/* View mode toggle */}
                        <div className="diff-toggle">
                            <button
                                className={`diff-toggle-btn${viewMode === 'side-by-side' ? ' diff-toggle-btn--active' : ''}`}
                                onClick={() => setViewMode('side-by-side')}
                            >
                                Side by Side
                            </button>
                            <button
                                className={`diff-toggle-btn${viewMode === 'unified' ? ' diff-toggle-btn--active' : ''}`}
                                onClick={() => setViewMode('unified')}
                            >
                                Unified
                            </button>
                        </div>

                        {/* Download patch */}
                        <button onClick={handleDownload} className="btn-primary diff-dl-btn">
                            <Download size={14} />
                            Patch
                        </button>
                    </div>
                </motion.div>

                {/* Diff panel */}
                <motion.div
                    key={`${fromVersion}-${toVersion}-${viewMode}`}
                    initial={{ opacity: 0, y: 6 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: 0.12 }}
                    className="glass diff-panel"
                >
                    {/* Panel header */}
                    <div className="diff-panel-header">
                        <FileCode size={14} className="diff-panel-icon" />
                        <span className="diff-panel-filename">token_contract.rs</span>
                        <span className="diff-panel-sep">{fromVersion} &rarr; {toVersion}</span>
                        <span className="diff-panel-meta">{stats.added + stats.removed} line{stats.added + stats.removed !== 1 ? 's' : ''} changed</span>
                    </div>

                    {fromVersion === toVersion ? (
                        <div className="diff-no-changes">
                            No changes &mdash; select different versions to compare.
                        </div>
                    ) : viewMode === 'side-by-side' ? (
                        <SideBySideView
                            pairs={pairs}
                            fromVersion={fromVersion}
                            toVersion={toVersion}
                            comments={comments}
                            activeComment={activeComment}
                            onToggle={handleToggleComment}
                            onAdd={handleAddComment}
                            onClose={() => setActiveComment(null)}
                        />
                    ) : (
                        <UnifiedView
                            diff={diff}
                            comments={comments}
                            activeComment={activeComment}
                            onToggle={handleToggleComment}
                            onAdd={handleAddComment}
                            onClose={() => setActiveComment(null)}
                        />
                    )}
                </motion.div>

                {/* Comment count summary */}
                {comments.length > 0 && (
                    <motion.div
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        className="diff-comment-summary glass"
                    >
                        <MessageSquare size={14} className="diff-comment-summary-icon" />
                        {comments.length} comment{comments.length !== 1 ? 's' : ''} on this diff
                    </motion.div>
                )}
            </div>
        </main>
    )
}
