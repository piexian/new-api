/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import { describe, expect, test } from 'bun:test';
import { buildNodeLocAuthorizationURL } from '../nodeloc';
import { getOAuthCallbackParams } from '../oauthCallback';

const status = {
  nodeloc_client_id: 'client&+',
  nodeloc_redirect_uri: 'https://gateway.example/oauth/nodeloc',
};

describe('NodeLoc authorization', () => {
  test('uses the server callback verbatim and requests only openid profile', () => {
    const url = buildNodeLocAuthorizationURL(
      status,
      'https://gateway.example',
      'state&+=',
    );
    expect(url.origin + url.pathname).toBe(
      'https://www.nodeloc.com/oauth-provider/authorize',
    );
    expect(Object.fromEntries(url.searchParams)).toEqual({
      client_id: 'client&+',
      redirect_uri: status.nodeloc_redirect_uri,
      response_type: 'code',
      scope: 'openid profile',
      state: 'state&+=',
    });
  });
  test('rejects an alternate site origin before authorization', () => {
    expect(() =>
      buildNodeLocAuthorizationURL(status, 'https://alias.example', 'state'),
    ).toThrow('请在配置的正式站点');
  });
  test('rejects missing or malformed configuration', () => {
    for (const callback of [
      '',
      'not-a-url',
      'ftp://gateway.example/oauth/nodeloc',
      'https://gateway.example/api/oauth/nodeloc',
      'https://gateway.example/oauth/nodeloc?x=1',
    ]) {
      expect(() =>
        buildNodeLocAuthorizationURL(
          { ...status, nodeloc_redirect_uri: callback },
          'https://gateway.example',
          'state',
        ),
      ).toThrow();
    }
    expect(() =>
      buildNodeLocAuthorizationURL(
        { ...status, nodeloc_client_id: '' },
        'https://gateway.example',
        'state',
      ),
    ).toThrow();
  });
});

describe('shared OAuth callbacks', () => {
  test('forwards provider denial and state without inventing a code', () => {
    const params = getOAuthCallbackParams(
      new URLSearchParams(
        'error=access_denied&error_description=No+thanks%26x&state=a%2Bb&unrelated=ignored',
      ),
    );
    expect(Object.fromEntries(params)).toEqual({
      error: 'access_denied',
      error_description: 'No thanks&x',
      state: 'a+b',
    });
    expect(
      new URLSearchParams(params.toString()).get('error_description'),
    ).toBe('No thanks&x');
  });
  test('preserves existing successful provider callbacks', () => {
    expect(
      Object.fromEntries(
        getOAuthCallbackParams(new URLSearchParams('code=a%26b%2B&state=s%3D')),
      ),
    ).toEqual({ code: 'a&b+', state: 's=' });
  });
});
