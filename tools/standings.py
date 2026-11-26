"""Snapshot Floodline's public Arena standings and provisional course points."""
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
from html import unescape
import json
import re
from urllib.request import urlopen


ARENA = 'https://arena.battlesnake.com'
FLOODLINE = 'eefbc819-2b24-4eec-b727-cf7441a9fee7'
PLAYER = '/users/bye-tranquility/a6b9eedb-f5e7-47ee-bc34-cec86ec2c56f'
BOARDS = {
    'Duels': '77212f22-20f5-4e0c-8db6-78844318a9bc',
    'Standard': 'd692fd01-5376-4060-8c4c-91dc2ecfe3ba',
    'Royale': '77ce41bf-12ad-4eac-bac2-b11939fb9b18',
    'Constrictor': '2997e28a-83a0-41e0-a203-d42108cfe450',
}
REFERENCES = {
    'Novice': '25fb8ade-c6db-40e5-ba50-6827fac1549c',
    'Apprentice': '99768fce-10ea-4529-bb33-09e59f936a32',
    'Journeyman': 'd53a9228-e6da-4b17-b362-a5c27f7b95ef',
    'Expert': '96f71577-d275-4d92-b11a-c457e2eeada4',
}


def fetch(path):
    with urlopen(ARENA + path, timeout=15) as response:
        return response.read().decode()


def standing(board_id, table, snake_id):
    for section in ('ranked', 'placement'):
        for row in table[section]:
            if row['battlesnake_id'] == snake_id:
                return {'status': section, 'rank': row['rank'] if section == 'ranked' else None,
                        'score': round(row['display_score'], 2), 'games': row['games_played']}
    # The public JSON lists at most 100 ranked snakes. The individual entry
    # page also reports the exact rank of snakes below that cut.
    profile = fetch(f'/battlesnakes/{snake_id}/profile')
    entry = re.search(rf'href="(/leaderboards/{board_id}/entries/[a-f0-9-]+)"', profile)
    if not entry:
        return {'status': 'absent', 'rank': None, 'score': None, 'games': 0}
    html = fetch(entry[1])
    fields = dict(re.findall(r'<div class="label">([^<]+)</div>\s*<div class="value(?: sm)?">([^<]+)', html))
    rank = fields['Rank']
    if rank != 'In Placement' and not re.fullmatch(r'#\d+', rank):
        raise ValueError(f'unrecognized Arena rank on {entry[1]}: {rank}')
    return {'status': 'ranked' if rank.startswith('#') else 'placement',
            'rank': int(rank[1:]) if rank.startswith('#') else None,
            'score': float(fields['Rating']), 'games': int(fields['Games'])}


def global_ranking():
    path = '/rankings'
    visited = set()
    cutoff = None
    while path not in visited:
        visited.add(path)
        html = fetch(path)
        mine = None
        for row in re.findall(r'<tr\b[^>]*>(.*?)</tr>', html, re.S):
            rank = re.search(r'<td class="rank">(\d+)</td>', row)
            score = re.search(r'<span class="total">([\d.]+)</span>', row)
            if not rank or not score:
                continue
            if int(rank[1]) == 30:
                cutoff = float(score[1])
            if f'href="{PLAYER}"' in row:
                mine = {'rank': int(rank[1]), 'score': float(score[1])}
        if mine:
            return mine, cutoff
        next_page = re.search(r'<a[^>]*href="(/rankings\?[^"#]+)"[^>]*>\s*Next', html)
        if not next_page:
            return None, cutoff
        path = unescape(next_page[1])
    raise ValueError('Arena rankings pagination repeated a page')


def main():
    paths = [f'/api/leaderboards/{board}/rankings' for board in BOARDS.values()]
    with ThreadPoolExecutor(max_workers=4) as pool:
        tables = dict(zip(BOARDS, map(json.loads, pool.map(fetch, paths))))
    own = {name: standing(board, tables[name], FLOODLINE) for name, board in BOARDS.items()}
    refs = {name: standing(BOARDS['Duels'], tables['Duels'], snake) for name, snake in REFERENCES.items()}
    me = own['Duels']['rank']
    beaten = [name for name, row in refs.items() if me is not None and row['rank'] is not None and me < row['rank']]
    player, cutoff = global_ranking()
    points = 15 * len(beaten) + (20 if beaten else 0) + (40 if player and player['rank'] <= 30 else 0)
    print(json.dumps({'as_of_utc': datetime.now(timezone.utc).isoformat(timespec='seconds'),
                      'profile': f'{ARENA}/battlesnakes/{FLOODLINE}/profile', 'boards': own,
                      'duel_references': refs, 'above': beaten, 'global_player': player,
                      'top_30_cutoff': cutoff, 'provisional_arena_points': points}, indent=2))


if __name__ == '__main__':
    main()
