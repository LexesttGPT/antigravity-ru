;(() => {
  'use strict';
  // Offline presentation-only localization. No IPC, network, storage or privileged API.
  const lex = __DICT__;
  const has = k => Object.prototype.hasOwnProperty.call(lex,k);
  const control = 'button,label,[role="button"],[role="menuitem"],[role="tab"],[role="option"],[role="heading"],h1,h2,h3,h4,nav a,aside a,[role="navigation"] a';
  const forbidden = 'input,textarea,[contenteditable]:not([contenteditable="false"]),pre,code,[role="alertdialog"],[role="alert"],[role="log"],[role="textbox"],[role="document"],[role="article"],article,[data-message-id],[data-testid*="message-content"],[data-testid*="chat-message"],[data-testid*="terminal-output"]';
  const settingsSignatures = ['Interface Layout','Global Permissions','Input Box','Permission preset','App version','Notification settings','Remote Control','Advanced Settings','General','Application','Appearance','Shortcuts','Notifications','Models','Models & Usage','Browser','Conversations','Not in Project','Orchestrator Settings','Built-in Browser','Layout Controls','Permissions','This computer','Chat Layout','Accessibility','Browser Actuation Permissions','Execute URLs','Download files','Remote Agents Data','Enable telemetry','Marketing emails','Steps to Reproduce','Attach Antigravity server logs'];
  const signature = new Set(settingsSignatures.flatMap(k=>[k,lex[k]]).filter(Boolean));
  const settingsWords = new Set(['Settings',lex.Settings,'General',lex.General,'Application',lex.Application,'Account',lex.Account,'Give Feedback',lex['Give Feedback'],'Browser Actuation Permissions',lex['Browser Actuation Permissions'],'Shortcuts',lex.Shortcuts]);
  const staticChrome = new Set(['Settings','Projects','Conversations','Recent','History','Automations','Customizations','Workspace','Workspaces','View','Window','File','Edit','Help','Skills','Rules','Workflows','Plugins','Connectors','Not in Project','Give Feedback']);
  const restricted = new Set(['Allow','Deny','Ask','Always allow','Always ask','Allow once','Auto-approve','Run','Stop','Continue','Delete','Remove','Install','Uninstall','Retry','Cancel','Open','Connect','Disconnect','Delete All Data','Submit','Request Review','Turbo']);
  const allowed = () => location.protocol === 'https:' && location.hostname === '127.0.0.1' && window.top === window;
  const normalized = s => s.trim().replace(/\s+/g,' ');
  const shortcutHints = new Set(['Search shortcuts...', 'Search shortcuts…','Search shortcuts','Search keyboard shortcuts...','Search keyboard shortcuts…'].flatMap(k=>[k,lex[k]]).filter(Boolean));
  const searchHints = new Set([...shortcutHints,'Search settings']);
  const feedbackHints = new Set(['Describe the bug you encountered...','Describe the bug you encountered…','Please list the steps to reproduce the issue']);
  const profileKeys = new Set(['Request Review','Default','Auto','Turbo','Every command requires approval.','Ask before sensitive operations.','Automatically decide if commands run.','Run all commands without restrictions.']);
  const profileLabels = new Set([...profileKeys].flatMap(k=>[k,lex[k]]));
  function profileRoot(e){for(let p=e, i=0;p&&p!==document.body&&i<7;p=p.parentElement,i++){if(p.closest(forbidden)|| (p.textContent||'').length>1500)break;const labels=new Set([...p.querySelectorAll('span,div,button,[role=option],[role=menuitem]')].filter(x=>x.childElementCount<=1&&!x.closest(forbidden)).map(x=>normalized(x.textContent||'')));if(['Request Review','Default','Turbo','Every command requires approval.','Run all commands without restrictions.'].every(k=>labels.has(k)||labels.has(lex[k])))return p;}return null;}
  const units = {day:['день','дня','дней'],hour:['час','часа','часов'],minute:['минуту','минуты','минут'],second:['секунду','секунды','секунд']};
  const duration = s => s.replace(/(\d+) (days?|hours?|minutes?|seconds?)/g,(_,n,u)=>{
    const a=units[u.replace(/s$/,'')], v=Number(n), tail=v%100;
    return n+' '+a[(tail>=11&&tail<=14)?2:(v%10===1?0:v%10>=2&&v%10<=4?1:2)];
  });
  const quota = /^You have used some of your (weekly|5-hour) limit, it will fully refresh in ((?:\d+ (?:days?|hours?|minutes?|seconds?))(?:, \d+ (?:days?|hours?|minutes?|seconds?)){0,3})\.$/;
  function settingsText(k) {
    const m=quota.exec(k);
    if(m) return 'Часть '+(m[1]==='weekly'?'недельного':'пятичасового')+' лимита использована. Он полностью обновится через '+duration(m[2])+'.';
    // Only specific fragments of the quota sentences, when React uses multiple text nodes.
    if(k==='You have used some of your weekly limit, it will fully refresh in')return 'Часть недельного лимита использована. Он полностью обновится через';
    if(k==='You have used some of your 5-hour limit, it will fully refresh in')return 'Часть пятичасового лимита использована. Он полностью обновится через';
    if(k==='Your plan:')return 'Ваш тариф:';
    const feedback=/^Send feedback as (.+)$/.exec(k);if(feedback)return 'Отправить отзыв от имени '+feedback[1];
    return null;
  }
  function translateText(s,settings) {
    const k=normalized(s);
    const v=has(k)?lex[k]:(settings?settingsText(k):null);
    return v===null?s:s.replace(s.trim(),v);
  }
  function settingsRoot(el) {
    // Check compact ancestors, never body/html. A settings panel has multiple independent labels.
    for(let p=el,depth=0;p && p!==document.body && p!==document.documentElement && depth<10;p=p.parentElement,depth++) {
      if (p.closest(forbidden)) return null;
      const txt=p.textContent||'';
      if(txt.length>40000) continue;
      const labels=new Set(); let anchor=false;
      for(const child of p.querySelectorAll('button,span,div,h1,h2,h3,h4,label,[role="heading"],[role="tab"]')) {
        if(child.childElementCount>3 || child.closest(forbidden)) continue;
        const k=(child.textContent||'').trim().replace(/\s+/g,' ');
        if(signature.has(k)) labels.add(k);
        if(settingsWords.has(k)) anchor=true;
        if(anchor&&labels.size>=3) {
          const dlg=p.closest('[role="dialog"],dialog');
          return dlg || p;
        }
      }
    }
    return null;
  }
  // Exact known command labels; do not localize arbitrary palette results or user text.
  const commandKeys = new Set([
    'Commands','Collapse All Folders','Expand All Folders','Select Next Conversation',
    'Select Previous Conversation','Move Conversation Up','Move Conversation Down',
    'Open Keyboard Shortcuts','Provide Feedback','Toggle Project Selector','Toggle Model Selector',
    'Toggle Voice Recording','Submit Prompt (Alternate to Enter)','Next Side Panel Tab',
    'Previous Side Panel Tab','Download Diagnostics','Toggle Between Light/Dark Themes',
    'Add a Gemini feature to my app','Open Conversation Picker','Open File Search',
    'Open Command Palette','New Conversation','Close Tab','Go Back','Go Forward',
    'New Tab','Reopen Closed Tab','Open Settings','Focus Input','Jump to Previous Prompt',
    'Jump to Next Prompt','Cancel Generation','Toggle Environment Selector','Mute Voice Mic',
    'Mute Voice Speaker','Find in Conversation','Find in Pane','Add to Chat/Quote',
    'Comment on Selection','Side Question Selection','Split Conversation Horizontally',
    'Split Conversation Vertically','Pin / Unpin Conversation','Archive This Conversation',
    'Toggle Sidebar','Toggle Side Panel','Toggle Maximize Side Panel','Toggle Overview Panel',
    'Toggle Terminal','Split Terminal','New Terminal Tab','Terminal: Add to Chat','Terminal: Select All',
    'Zoom In','Zoom Out','Reset Zoom','Create Project','Command Palette','Automations'
  ]);
  const shortcutKeys = new Set([...commandKeys].filter(k=>!['Commands','Collapse All Folders','Expand All Folders','Download Diagnostics','Add a Gemini feature to my app','Create Project','Command Palette','Provide Feedback','Toggle Between Light/Dark Themes'].includes(k)));
  const paletteHints = new Set(['Search for commands...','Search for commands…',lex['Search for commands...'],lex['Search for commands…']]);
  const paletteHeaders = new Set(['Commands',lex.Commands]);
  const workspaceKeys = new Set(['New Conversation','Create Project','Command Palette']);
  const workspaceLabels = new Set([...workspaceKeys].flatMap(k=>[k,lex[k]]));
  const rememberedPalettes = new WeakMap();
  function paletteRoot(input) {
    if(!paletteHints.has(input.getAttribute('placeholder')) || input.parentElement.closest(forbidden)) return null;
    const cached=rememberedPalettes.get(input);
    if(cached && cached.isConnected && cached.contains(input))return cached;
    const enclosing=input.closest('[role="dialog"],dialog');
    for(let p=input.parentElement,depth=0;p && p!==document.body && p!==document.documentElement && depth<9;p=p.parentElement,depth++) {
      if(p.closest(forbidden) || (enclosing && !enclosing.contains(p))) break;
      if((p.textContent||'').length>24000) break;
      const label=[...p.querySelectorAll('button,span,div,[role="button"],[role="combobox"]')].some(e=>e.childElementCount<=2 && !e.closest(forbidden) && paletteHeaders.has(normalized(e.textContent||'')));
      if(!label) continue;
      // Prefer the enclosing dialog when it actually holds the identified command header.
      if(enclosing) {rememberedPalettes.set(input,enclosing);return enclosing;}
      // Without dialog semantics, look for an identifiable list or known command rows.
      const commandTexts=new Set([...commandKeys].flatMap(k=>[k,lex[k]]));
      if(p.querySelector('[role="listbox"],[role="option"],[role="menu"],[role="menuitem"],[cmdk-list]') || [...p.querySelectorAll('button,span,div')].some(e=>e.childElementCount<=2 && commandTexts.has(normalized(e.textContent||'')) && !paletteHeaders.has(normalized(e.textContent||'')))) {rememberedPalettes.set(input,p);return p;}
    }
    return null;
  }
  function workspaceRoot(el) {
    for(let p=el,depth=0;p && p!==document.body && p!==document.documentElement && depth<6;p=p.parentElement,depth++) {
      if(p.closest(forbidden) || p.closest('[role="dialog"],dialog')) return null;
      if((p.textContent||'').length>600) break;
      const labels=new Set([...p.querySelectorAll('button,span,div,[role="menuitem"]')].filter(e=>e.childElementCount<=2&&!e.closest(forbidden)).map(e=>normalized(e.textContent||'')).filter(k=>workspaceLabels.has(k)));
      if([...workspaceKeys].every(k=>labels.has(k)||labels.has(lex[k])))return p;
    }
    return null;
  }
  const automationKeys = new Set(['Automations','Create with Prompt','New','No automations yet',
    "Run a prompt on a schedule, like a morning summary of yesterday's changes. Runs show up here.",
    'Run a prompt on a schedule, like a morning summary of yesterday’s changes. Runs show up here.',
    "Run a prompt on a schedule, like a morning summary of yesterday's changes.",'Runs show up here.']);
  const automationLabels=k=>new Set([k,lex[k]]);
  function automationRoot(title) {
    if(title.closest(forbidden+',nav,aside,[role="navigation"],[role="dialog"],dialog'))return null;
    let header=null;
    for(let p=title.parentElement,i=0;p&&p!==document.body&&p!==document.documentElement&&i<9;p=p.parentElement,i++) {
      if(p.closest(forbidden+',nav,aside,[role="navigation"],[role="dialog"],dialog'))break;
      if((p.textContent||'').length>20000)break;
      const labels=new Set([...p.querySelectorAll('button,[role="button"]')].filter(e=>!e.closest(forbidden)).map(e=>normalized(e.textContent||'')));
      if([...automationLabels('Create with Prompt')].some(k=>labels.has(k)) && [...automationLabels('New')].some(k=>labels.has(k))) {
        if(!header)header=p;
        const empty=[...p.querySelectorAll('h1,h2,h3,h4,div,span,p')].some(e=>e.childElementCount<=1&&!e.closest(forbidden+',nav,aside,[role="dialog"],dialog')&&automationLabels('No automations yet').has(normalized(e.textContent||'')));
        if(empty)return p;
      }
    }
    return header;
  }
  // Keep English command labels in textContent for native search. Only paint Russian
  // text for simple leaf labels; do not replace app controls or change command IDs.
  const painted = new WeakSet();
  function paintCommand(e,k) {
    if(!has(k) || e.childElementCount!==0 || e.closest(forbidden))return;
    if(!painted.has(e)) {
      const cs=window.getComputedStyle(e);
      e.style.setProperty('--ag-ru-command-size',cs.fontSize);
      painted.add(e);
    }
    if(e.getAttribute('data-ag-ru-command-label')!==lex[k]) e.setAttribute('data-ag-ru-command-label',lex[k]);
  }
  function clearPaint(e) {
    if(!painted.has(e))return;
    e.removeAttribute('data-ag-ru-command-label');e.style.removeProperty('--ag-ru-command-size');painted.delete(e);
  }
  function installPaletteStyle() {
    if(document.getElementById('ag-ru-palette-style'))return;
    const style=document.createElement('style');style.id='ag-ru-palette-style';
    style.textContent=`[data-ag-ru-command-label]{font-size:0!important} [data-ag-ru-command-label]::after{content:attr(data-ag-ru-command-label);font-size:var(--ag-ru-command-size,14px);white-space:inherit}
[data-ag-ru-search-helper]{position:fixed;z-index:2147483000;box-sizing:border-box;padding:8px;background:#1c1e23;color:#e8eaed;border:1px solid #686c76;border-radius:8px;box-shadow:0 8px 26px #0007;max-height:260px;overflow:auto;font:13px system-ui,sans-serif;text-align:left}
[data-ag-ru-search-helper] .ag-ru-search-note{padding:5px 7px;color:#b5bac5;font-size:12px}
[data-ag-ru-search-helper] button{box-sizing:border-box;display:block;width:100%;padding:6px 8px;margin:2px 0;text-align:left;background:transparent;color:inherit;border:0;border-radius:4px;cursor:pointer;font:inherit;white-space:normal}
[data-ag-ru-search-helper] button:hover,[data-ag-ru-search-helper] button:focus-visible{background:#3d424c;outline:1px solid #a4b8ee}`;
    (document.head||document.documentElement).appendChild(style);
  }
  const ruFold=s=>s.toLocaleLowerCase('ru').replace(/ё/g,'е');
  const commandCandidates=[...commandKeys].filter(k=>has(k)&&k!=='Commands').map(k=>({en:k,ru:lex[k]}));
  const helpers=new Map();
  function removeHelper(input) {const old=helpers.get(input);if(old){old.node.remove();helpers.delete(input);}}
  function nativeQuery(input,english) {
    // This only populates the existing search field. Never fires Enter or invokes a command.
    const setter=Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,'value').set;
    setter.call(input,english);
    input.dispatchEvent(new Event('input',{bubbles:true,composed:true}));
    input.focus();
  }
  function syncHelper(input,root,isShortcut=false) {
    const q=normalized(input.value||'');
    if(!/\p{Script=Cyrillic}/u.test(q) || q.length>120){removeHelper(input);return;}
    const parts=ruFold(q).split(/\s+/).filter(Boolean);
    const matches=commandCandidates.filter(x=>(!isShortcut||shortcutKeys.has(x.en))&&parts.every(part=>ruFold(x.ru+' '+x.en).includes(part)));
    matches.sort((a,b)=>Number(ruFold(b.ru)===ruFold(q))-Number(ruFold(a.ru)===ruFold(q)) || Number(ruFold(b.ru).startsWith(ruFold(q)))-Number(ruFold(a.ru).startsWith(ruFold(q))));
    let state=helpers.get(input);
    if(state&&(!state.node.isConnected||state.root!==root)){removeHelper(input);state=null;}
    if(!state) {
      const node=document.createElement('div');node.setAttribute('data-ag-ru-search-helper','');
      node.setAttribute('role','region');node.setAttribute('aria-label',isShortcut?'Русский поиск горячих клавиш':'Русский поиск команд');
      root.appendChild(node);state={node,root,q:null};helpers.set(input,state);
    }
    const rect=input.getBoundingClientRect(), box=state.node;
    const w=Math.min(Math.max(rect.width,220),Math.max(100,window.innerWidth-16));
    box.style.width=w+'px';box.style.left=Math.max(8,Math.min(rect.left,window.innerWidth-w-8))+'px';
    box.style.top=Math.max(8,Math.min(rect.bottom+4,window.innerHeight-268))+'px';
    if(state.q===q)return;
    state.q=q;box.replaceChildren();
    const note=document.createElement('div');note.className='ag-ru-search-note';
    note.textContent=matches.length?(isShortcut?'Найденные горячие клавиши — выберите для поиска:':'Найденные команды — выберите для поиска в Antigravity:'):'Русских совпадений нет. Попробуйте английское название.';
    box.appendChild(note);
    for(const item of matches.slice(0,12)) {
      const b=document.createElement('button');b.type='button';b.textContent=item.ru+' · '+item.en;
      b.addEventListener('pointerdown',e=>e.preventDefault());
      b.addEventListener('click',e=>{e.preventDefault();e.stopPropagation();if(input.isConnected){removeHelper(input);nativeQuery(input,item.en);}});
      box.appendChild(b);
    }
    if(matches.length>12){const more=document.createElement('div');more.className='ag-ru-search-note';more.textContent='Уточните запрос: совпадений '+matches.length;box.appendChild(more);}
  }
  function run() {
    if(!allowed() || !document.body) return;
    installPaletteStyle();
    let scheduled=false;
    const update=()=>{
      scheduled=false;
      if(!allowed()) return;
      // Cache panel classification once per scan; do not translate arbitrary message/document text.
      const panels=new Set();
      for(const e of document.querySelectorAll('[role="dialog"],dialog,button,[role="tab"],h1,h2,h3,[role="heading"],span,div')) {
        if(e.closest(forbidden)) continue;
        if(e.childElementCount>2) continue;
        const t=(e.textContent||'').trim();
        if(!settingsWords.has(t)) continue;
        const p=settingsRoot(e);
        if(p) panels.add(p);
      }
      const inSettings=e=>{for(const p of panels) if(p.contains(e)) return true;return false;};
      const palettes=new Set(),paletteInputs=new Map(),shortcutInputs=new Map(),shortcutPanels=new Set(),profilePanels=new Set(),automationPanels=new Set();
      for(const input of document.querySelectorAll('input[placeholder]')) {const p=paletteRoot(input);if(p){palettes.add(p);paletteInputs.set(input,p);}}
      for(const input of document.querySelectorAll('input[placeholder]'))if(shortcutHints.has(input.getAttribute('placeholder')) && inSettings(input) && !input.parentElement.closest(forbidden)){const p=[...panels].find(p=>p.contains(input));if(p){shortcutInputs.set(input,p);shortcutPanels.add(p);}}
      for(const input of [...helpers.keys()])if(!paletteInputs.has(input)&&!shortcutInputs.has(input))removeHelper(input);
      if(panels.size)for(const e of document.querySelectorAll('span,div,[role=option],[role=menuitem]')){if(e.childElementCount>1||!profileLabels.has(normalized(e.textContent||'')))continue;const p=profileRoot(e);if(p)profilePanels.add(p);}
      for(const e of document.querySelectorAll('h1,h2,h3,h4,[role="heading"],div,span')) {
        if(e.childElementCount>1 || !automationLabels('Automations').has(normalized(e.textContent||'')))continue;
        const p=automationRoot(e);if(p)automationPanels.add(p);
      }
      const menus=new Set();
      for(const e of document.querySelectorAll('button,span,div,[role="menuitem"]')) {
        if(e.childElementCount>2 || !workspaceLabels.has(normalized(e.textContent||''))) continue;
        const p=workspaceRoot(e);if(p)menus.add(p);
      }
      const inside=(e,roots)=>[...roots].some(p=>p.contains(e) && (!e.closest('[role="dialog"],dialog') || e.closest('[role="dialog"],dialog')===p || e.closest('[role="dialog"],dialog').contains(p)));
      const inPalette=e=>inside(e,palettes);
      const inShortcut=e=>inside(e,shortcutPanels);
      const inProfile=e=>inside(e,profilePanels);
      const inWorkspace=e=>inside(e,menus);
      const inAutomation=e=>inside(e,automationPanels);
      for(const e of document.querySelectorAll('[data-ag-ru-command-label]')) {
        const k=normalized(e.textContent||'');
        if((!inPalette(e)||!commandKeys.has(k)||k==='Commands')&&(!inShortcut(e)||!shortcutKeys.has(k))||e.childElementCount!==0)clearPaint(e);
      }
      const can=e=>{
        if(!e || e.closest(forbidden)) return false;
        const dlg=e.closest('[role="dialog"],dialog');
        if(dlg && !inSettings(e)) return false;
        // Never translate nested confirmation dialogs inside a settings panel.
        if(dlg && ![...panels].some(p=>dlg.contains(p) || p===dlg)) return false;
        return true;
      };
      const walker=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);
      let n; while((n=walker.nextNode())) {
        const e=n.parentElement;if(!e || e.closest(forbidden))continue;
        const k=normalized(n.nodeValue);
        if(e.closest('[data-ag-ru-search-helper]'))continue;
        if(inPalette(e)) {
          if(commandKeys.has(k)&&k==='Commands'&&has(k)) {const v=translateText(n.nodeValue,false);if(v!==n.nodeValue)n.nodeValue=v;}
          else if(commandKeys.has(k)&&normalized(e.textContent||'')===k)paintCommand(e,k);
          // Do not mutate the original labels of command results or any unknown palette text.
          continue;
        }
        if(inProfile(e)&&profileKeys.has(k)){const v=translateText(n.nodeValue,false);if(v!==n.nodeValue)n.nodeValue=v;continue;}
        if(inShortcut(e)&&shortcutKeys.has(k)&&normalized(e.textContent||'')===k&&e.childElementCount===0){paintCommand(e,k);continue;}
        if((inWorkspace(e)&&workspaceKeys.has(k)) || (inAutomation(e)&&automationKeys.has(k)&&(!['New','Create with Prompt'].includes(k)||e.closest('button,[role="button"]')))) {
          if(has(k)) {const v=translateText(n.nodeValue,false);if(v!==n.nodeValue)n.nodeValue=v;}
          continue;
        }
        if(!can(e))continue;
        const settings=inSettings(e);
        if(settings && /^\d+ (?:days?|hours?|minutes?|seconds?)(?:, \d+ (?:days?|hours?|minutes?|seconds?)){0,3}\.?$/.test(k)) {
          let parent=e;
          for(let j=0;parent&&j<3;j++,parent=parent.parentElement) {
            const t=normalized(parent.textContent||'');
            if(quota.test(t)||/^Часть (?:недельного|пятичасового) лимита использована\. Он полностью обновится через /.test(t)) {
              n.nodeValue=n.nodeValue.replace(n.nodeValue.trim(),duration(k));break;
            }
          }
          continue;
        }
        if(!has(k)&&(!settings||settingsText(k)===null))continue;
        const chrome=e.closest('nav,aside,header,[role="navigation"],[role="toolbar"],[role="menubar"],[role="menu"],[role="tablist"]');
        const c=e.closest(control);
        if(!settings && ((!c && !(chrome && staticChrome.has(k))) || restricted.has(k))) continue;
        if(!settings && !chrome && c && c.matches('h1,h2,h3,h4,[role="heading"]')) continue;
        if(!settings && !chrome && (!c || !c.matches('button,[role="button"],[role="tab"],[role="menuitem"],h1,h2,h3,h4,[role="heading"]'))) continue;
        const v=translateText(n.nodeValue,settings);if(v!==n.nodeValue)n.nodeValue=v;
      }
      for(const [input,root] of paletteInputs)syncHelper(input,root);
      for(const [input,root] of shortcutInputs)syncHelper(input,root,true);
      for(const e of document.querySelectorAll(control+',input[placeholder],textarea[placeholder]')) {
        // Only translate display metadata, not user input values or accessible control names.
        if(e.matches('input,textarea')) {
          const k=e.getAttribute('placeholder');
          if(k==='Ask anything, @ to mention, / for actions' && !e.closest('[role="dialog"],[role="alertdialog"],[role="log"],article,[data-message-id]')) e.setAttribute('placeholder',lex[k]);
          else if((searchHints.has(k)||feedbackHints.has(k))&&inSettings(e)&&has(k)&&!e.parentElement.closest(forbidden))e.setAttribute('placeholder',lex[k]);
          else if(paletteHints.has(k)&&inPalette(e)&&has(k)&&!e.parentElement.closest(forbidden))e.setAttribute('placeholder',lex[k]);
          continue;
        }
        if(!can(e)||!inSettings(e))continue;
        const t=e.getAttribute('title');if(t&&has(t)&&!restricted.has(t))e.setAttribute('title',lex[t]);
      }
    };
    const schedule=()=>{if(!scheduled){scheduled=true;window.requestAnimationFrame(update);}};
    document.addEventListener('input',e=>{if(e.target instanceof HTMLInputElement && (paletteHints.has(e.target.getAttribute('placeholder'))||shortcutHints.has(e.target.getAttribute('placeholder'))))schedule();},true);
    window.addEventListener('resize',schedule);
    update();
    new MutationObserver(schedule).observe(document.body,{childList:true,subtree:true,characterData:true,attributes:true,attributeFilter:['placeholder']});
  }
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',run,{once:true});else run();
})();
