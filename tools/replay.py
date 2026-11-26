"""Export the last decision positions of one snake from a public Arena replay."""
import argparse
from collections import deque
from concurrent.futures import ThreadPoolExecutor
import gzip
import json
from pathlib import Path
from urllib.parse import urlencode
from uuid import UUID

from standings import fetch


def ending(game, author, snake_id, count):
    turns = deque(maxlen=count)
    offset = 0
    multiplayer = False
    while True:
        page = json.loads(fetch(f'/api/games/{game}/frames?offset={offset}&limit=100'))['Frames']
        if not page:
            break
        if offset == 0:
            multiplayer = len(page[0]['Snakes']) > 1
            if not snake_id:
                owned = [s['ID'] for s in page[0]['Snakes'] if s['Author'] == author]
                if len(owned) != 1:
                    raise ValueError(f'found {len(owned)} opening snakes for {author}, specify the in-game --you ID')
                snake_id = owned[0]
        for frame in page:
            if frame['Turn'] != offset:
                raise ValueError(f'nonconsecutive frames at turn {offset}')
            me = next((s for s in frame['Snakes'] if s['ID'] == snake_id), None)
            if me is None or me.get('Death') or me.get('EliminatedCause'):
                if not turns:
                    raise ValueError('the selected snake has no decision positions')
                return snake_id, list(turns)
            alive = sum(not s.get('Death') and not s.get('EliminatedCause') for s in frame['Snakes'])
            if multiplayer and alive <= 1 and frame['Turn'] > 0:
                return snake_id, list(turns)
            turns.append(frame['Turn'])
            offset += 1
        if len(page) < 100:
            break
    if not turns:
        raise ValueError('no decision positions found')
    return snake_id, list(turns)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('game', type=UUID)
    parser.add_argument('--author', default='bye-tranquility')
    parser.add_argument('--you', type=UUID, help='per-game snake ID when an author has multiple entries')
    parser.add_argument('--last', type=int, default=12)
    parser.add_argument('--out', type=Path, required=True, help='new .jsonl or .jsonl.gz file')
    args = parser.parse_args()
    if args.last <= 0 or args.out.exists():
        parser.error('provide a positive --last count and a new output path')
    snake_id, turns = ending(args.game, args.author, str(args.you) if args.you else None, args.last)

    def position(turn):
        query = urlencode({'turn': turn, 'you': snake_id})
        state = json.loads(fetch(f'/api/games/{args.game}/move-request?{query}'))
        if state['game']['id'] != str(args.game) or state['turn'] != turn or state['you']['id'] != snake_id:
            raise ValueError(f'Arena returned a different position for turn {turn}')
        return json.dumps(state, ensure_ascii=False, separators=(',', ':')) + '\n'

    with ThreadPoolExecutor(max_workers=3) as pool:
        data = ''.join(pool.map(position, turns)).encode()
    if args.out.suffix == '.gz':
        data = gzip.compress(data, mtime=0)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open('xb') as dest:
        dest.write(data)
    print(f'{len(turns)} positions, turns {turns[0]}–{turns[-1]}, {args.out}')


if __name__ == '__main__':
    main()
