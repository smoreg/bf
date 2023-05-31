//nolint:gosec
package main

import "math/rand"

type fakeJokeRepo struct {
	jokes []string
}

func (r *fakeJokeRepo) GetARandomJoke() string {
	if r.jokes != nil {
		return r.jokes[rand.Intn(len(r.jokes))]
	}

	return "Why did the tomato turn red? Because it saw the salad dressing!"
}

func (r *fakeJokeRepo) GetARandomGoodJoke() string {
	if r.jokes != nil {
		return r.jokes[rand.Intn(len(r.jokes))]
	}

	return "What do you call a fish with no eyes? A fsh."
}

func (r *fakeJokeRepo) SaveJoke(joke string) {
	r.jokes = append(r.jokes, joke)
}
